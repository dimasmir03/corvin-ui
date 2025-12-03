package app

import (
	"log"
	"strconv"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers"
	"vpnpanel/internal/jobs"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
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

	Cron *cron.Cron
}

func NewServer() *Server {
	settingsRepo := repository.NewSettingsRepo(db.DB)

	serverService := repository.NewServerRepo(db.DB)

	keys := []string{
		"minio_endpoint",
		"minio_access_key",
		"minio_secret_key",
		"minio_ssl",
	}

	values, err := settingsRepo.GetKeys(keys...)
	if err != nil {
		log.Fatalf("Failed to get settings: %v", err)
	}
	minioSSL, err := strconv.ParseBool(values["minio_ssl"])
	if err != nil {
		log.Fatalf("Failed to parse minio_ssl to bool: %v", err)
	}
	minioClient, err := storage.NewMinioClient(
		values["minio_endpoint"],
		values["minio_access_key"],
		values["minio_secret_key"],
		"complaints",
		minioSSL,
	)
	if err != nil {
		log.Fatal(err)
	}

	storageRepo := repository.NewStorageRepo(minioClient)
	teleRepo := repository.NewTelegramRepo(db.DB)
	complaintRepo := repository.NewComplaintRepo(db.DB)
	userRepo := repository.NewUserRepo(db.DB)
	serversRepo := repository.NewServerRepo(db.DB)
	vpnRepo := repository.NewVpnRepo(db.DB)

	s := &Server{
		ServersService: serverService,
		StorageRepo:    storageRepo,

		TelegramController:   handlers.NewTelegramController(storageRepo, teleRepo),
		ComplaintsController: handlers.NewComplaintsController(complaintRepo),
		UserController:       handlers.NewUserController(userRepo),
		ServersController:    handlers.NewServersController(serversRepo),
		PanelController:      handlers.NewPanelController(),
		VpnController:        handlers.NewVpnController(vpnRepo),

		Cron: cron.New(cron.WithSeconds()),
	}

	s.Router = s.Routes()
	return s
}

func (s *Server) CronStart() {
	if s.ServersService == nil {
		log.Println("⚠️ ServersService is nil — Cron jobs skipped")
		return
	}

	s.Cron.AddJob("@every 5s", jobs.NewCollectTotalOnlineJob(s.ServersService))

	s.Cron.AddFunc("@daily", func() {
		s.ServersService.ClearStats()
	})

	s.Cron.Start()
}
