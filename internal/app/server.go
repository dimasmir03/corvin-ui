package app

import (
	"vpnpanel/internal/db"
	"vpnpanel/internal/jobs"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/robfig/cron/v3"
)

type Server struct {
	Router         *gin.Engine
	Store          *sessions.CookieStore
	ServersService *repository.ServerRepo
	Cron           *cron.Cron
}

func NewServer() *Server {
	db.Init()

	return &Server{
		Router:         Routes(),
		ServersService: repository.NewServerRepo(db.DB),
		Cron:           cron.New(cron.WithSeconds()),
	}
}

func (s *Server) CronStart() {
 	s.Cron.AddJob("@every 5s", jobs.NewCollectTotalOnlineJob(*s.ServersService))

	s.Cron.AddFunc("@daily", func() {
		s.ServersService.ClearStats()
	})

	s.Cron.Start()
}
