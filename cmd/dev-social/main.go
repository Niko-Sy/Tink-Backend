// Run an entirely local seeded development server, cleaned up on Ctrl-C.
package main

import (
	"chatroombackend/api/websocketmsg"
	"chatroombackend/internal/testenv"
	"chatroombackend/middleware"
	"chatroombackend/server"
	social "chatroombackend/service/social"
	"chatroombackend/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	env, err := testenv.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer env.Close()
	if err = testenv.Migrate(env.DB, 7); err != nil {
		log.Fatal(err)
	}
	ids, err := testenv.Seed(env.DB)
	if err != nil {
		log.Fatal(err)
	}
	cfg := middleware.DefaultDBConfig()
	cfg.DSN = env.DSN
	manager, err := middleware.NewDBManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		log.Fatal(err)
	}
	middleware.JWTSecretKey = []byte(hex.EncodeToString(key))
	uploadDir, err := os.MkdirTemp("", "tink-social-uploads-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(uploadDir)
	utils.InitImageUploader(&utils.ImageConfig{UploadDir: filepath.Join(uploadDir, "images"), MaxSize: 5 << 20, AllowedTypes: []string{"image/png", "image/jpeg"}, URLPrefix: "/static/images", BaseURL: "http://127.0.0.1:18080"})
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	done := server.StartBackground(jobCtx, manager, social.Capabilities{Friends: true, DirectMessages: true, RoomInvitations: true})
	router := server.NewRouter(manager, social.Capabilities{Friends: true, DirectMessages: true, RoomInvitations: true})
	httpServer := &http.Server{Addr: "127.0.0.1:18080", Handler: router, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("Local API: http://127.0.0.1:18080; disposable accounts alice/bob/carol; password Testpass123!; user IDs %v\n", ids)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	_ = websocketmsg.Shutdown(ctx)
	jobCancel()
	<-done
}
