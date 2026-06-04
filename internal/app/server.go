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
	"vpnpanel/internal/repository"
	"vpnpanel/internal/storage"

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

	ResultConsumer *rabbitmq.Consumer
	Cron           *cron.Cron

	Config config.Config
}

func NewServer(cfg config.Config) *Server {
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
	auditLogger := audit.NewLogger(repository.NewAuditRepo(db.DB))
	jobService := jobsvc.NewService(
		repository.NewJobsRepo(db.DB),
		serversRepo,
		auditLogger,
		broker.GlobalProducer,
	)

	s := &Server{
		ServersService: serverService,
		StorageRepo:    storageRepo,

		TelegramController:   handlers.NewTelegramController(storageRepo, teleRepo, jobService, auditLogger),
		ComplaintsController: handlers.NewComplaintsController(complaintRepo),
		UserController:       handlers.NewUserController(userRepo, auditLogger),
		ServersController:    handlers.NewServersController(serversRepo, jobService, auditLogger),
		PanelController:      handlers.NewPanelController(),
		VpnController:        handlers.NewVpnController(vpnRepo),
		MediaController:      handlers.NewMediaController(storageRepo),
		JobsController:       handlers.NewJobsController(jobService),

		Cron:   cron.New(cron.WithSeconds()),
		Config: cfg,
	}

	if broker.GlobalProducer != nil {
		consumer, err := broker.GlobalProducer.StartResultConsumer(cfg.RabbitMQ.ResultQueue, func(event broker.JobResultEvent) error {
			_, _, err := jobService.ApplyResult(event)
			return err
		})
		if err != nil {
			log.Printf("failed to start RabbitMQ result consumer: %v", err)
		} else {
			s.ResultConsumer = consumer
		}
	}

	s.Router = s.Routes()
	return s
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
	if s.ResultConsumer != nil {
		s.ResultConsumer.Close()
	}
	if s.Cron != nil {
		s.Cron.Stop()
	}
}
