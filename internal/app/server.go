package app

import (
	"log"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/config"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers"
	"vpnpanel/internal/jobs"
	"vpnpanel/internal/jobsvc"
	projectlogger "vpnpanel/internal/logger"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"
	"vpnpanel/internal/storage"
	"vpnpanel/internal/telegrambot"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/wagslane/go-rabbitmq"
)

type Server struct {
	Router *gin.Engine

	StorageRepo    *repository.StorageRepo
	ServersService *repository.ServerRepo

	TelegramController   *handlers.TelegramController
	ComplaintsController *handlers.ComplaintsController
	UserController       *handlers.UserController
	ServersController    *handlers.ServersController
	PanelController      *handlers.PanelController
	VpnController        *handlers.VpnController
	MediaController      *handlers.MediaController
	JobsController       *handlers.JobsController

	telegramBot      *telegrambot.Bot
	telegramNotifier *telegrambot.Notifier

	ResultConsumer *rabbitmq.Consumer
	Cron           *cron.Cron

	Config config.Config
}

func NewServer(cfg config.Config) (*Server, error) {
	serverService := repository.NewServerRepo(db.DB)

	minioClient, err := storage.NewMinioClient(
		cfg.MinIO.Endpoint,
		cfg.MinIO.AccessKey,
		cfg.MinIO.SecretKey,
		cfg.MinIO.Bucket,
		cfg.MinIO.UseSSL,
	)
	if err != nil {
		log.Fatal(err)
	}

	teleRepo := repository.NewTelegramRepo(db.DB)
	complaintRepo := repository.NewComplaintRepo(db.DB)
	userRepo := repository.NewUserRepo(db.DB)
	serversRepo := repository.NewServerRepo(db.DB)
	vpnRepo := repository.NewVpnRepo(db.DB)
	storageRepo := repository.NewStorageRepo(minioClient)
	technicalLogger := projectlogger.Default()
	auditLogger := audit.NewLogger(repository.NewAuditRepo(db.DB))
	usersService := service.NewUsersService(teleRepo, auditLogger)
	jobService := jobsvc.NewService(
		repository.NewJobsRepo(db.DB),
		serversRepo,
		auditLogger,
		broker.GlobalProducer,
	)
	vpnService := service.NewVPNService(vpnRepo, teleRepo, jobService, auditLogger)
	supportService := service.NewSupportService(teleRepo, complaintRepo)

	tgBot, err := telegrambot.New(cfg.Telegram, telegrambot.Deps{
		Users:   usersService,
		VPN:     vpnService,
		Support: supportService,
		Logger:  technicalLogger,
	})
	if err != nil {
		return nil, err
	}
	var tgNotifier *telegrambot.Notifier
	if tgBot != nil {
		tgNotifier = tgBot.Notifier()
		tgBot.Start()
	}

	s := &Server{
		ServersService: serverService,
		StorageRepo:    storageRepo,

		TelegramController:   handlers.NewTelegramController(storageRepo, teleRepo, usersService, vpnService, jobService, auditLogger),
		ComplaintsController: handlers.NewComplaintsController(complaintRepo),
		UserController:       handlers.NewUserController(userRepo, auditLogger),
		ServersController:    handlers.NewServersController(serversRepo, jobService, auditLogger),
		PanelController:      handlers.NewPanelController(),
		VpnController:        handlers.NewVpnController(vpnRepo),
		MediaController:      handlers.NewMediaController(storageRepo),
		JobsController:       handlers.NewJobsController(jobService),

		telegramBot:      tgBot,
		telegramNotifier: tgNotifier,

		Cron:   cron.New(cron.WithSeconds()),
		Config: cfg,
	}

	if broker.GlobalProducer != nil {
		consumer, err := broker.GlobalProducer.StartResultConsumer(cfg.RabbitMQ.ResultQueue, func(event broker.JobResultEvent) error {
			_, job, err := jobService.ApplyResult(event)
			if err != nil {
				technicalLogger.Error("job result apply failed", err, "job_id", event.JobID, "batch_id", event.BatchID)
				return err
			}

			notification, err := vpnService.ApplyAgentCreateResult(job, event)
			if err != nil {
				technicalLogger.Error("vpn agent result apply failed", err, "job_id", event.JobID, "batch_id", event.BatchID)
				return err
			}

			if notification != nil {
				technicalLogger.Info("agent vpn result applied", "tg_id", notification.TgID, "protocol", notification.Protocol, "job_id", event.JobID, "batch_id", event.BatchID)
				technicalLogger.Info("vpn link saved", "tg_id", notification.TgID, "protocol", notification.Protocol, "job_id", event.JobID, "batch_id", event.BatchID)
				if tgNotifier != nil {
					if err := tgNotifier.SendVPNReady(notification.TgID, notification.Link); err != nil {
						technicalLogger.Error("telegram vpn ready notification failed", err, "tg_id", notification.TgID, "protocol", notification.Protocol)
					}
				}
			}

			return nil
		})
		if err != nil {
			log.Printf("failed to start RabbitMQ result consumer: %v", err)
		} else {
			s.ResultConsumer = consumer
		}
	}

	s.Router = s.Routes()
	return s, nil
}

func (s *Server) CronStart() {
	if s.ServersService == nil {
		log.Println("⚠️ ServersService is nil — Cron jobs skipped")
		return
	}

	collectInterval := s.Config.OnlineCollectInterval
	if _, err := time.ParseDuration(collectInterval); err != nil {
		log.Printf("invalid ONLINE_COLLECT_INTERVAL %q, fallback to 30s: %v", collectInterval, err)
		collectInterval = "30s"
	}
	if _, err := s.Cron.AddJob("@every "+collectInterval, jobs.NewCollectTotalOnlineJob(s.ServersService)); err != nil {
		log.Printf("failed to schedule online collect job: %v", err)
	}

	s.Cron.AddFunc("@daily", func() {
		s.ServersService.ClearStats()
	})

	s.Cron.Start()
}

func (s *Server) Close() {
	if s.telegramBot != nil {
		s.telegramBot.Stop()
	}
	if s.ResultConsumer != nil {
		s.ResultConsumer.Close()
	}
	if s.Cron != nil {
		s.Cron.Stop()
	}
}
