package router

import (
	"cw7/internal/handlers"
	"cw7/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(
	mode string,
	withdrawH *handlers.WithdrawHandler,
	reconcileH *handlers.ReconcileHandler,
) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.AccessLogger())

	health := r.Group("/health")
	{
		health.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}

	v1 := r.Group("/api/v1")
	{
		withdraw := v1.Group("/withdraw")
		{
			withdraw.POST("/apply", withdrawH.Apply)
			withdraw.GET("/no/:no", withdrawH.GetByNo)
			withdraw.GET("/driver/list", withdrawH.ListByDriver)
			withdraw.GET("/today/cache", withdrawH.ListCachedToday)
		}
		reconcile := v1.Group("/reconcile")
		{
			reconcile.POST("/check", reconcileH.Check)
			reconcile.GET("/result/:batch_no", reconcileH.GetResult)
		}
	}
	return r
}
