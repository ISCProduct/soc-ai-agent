package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupScheduleRoutes(api *echo.Group, scheduleController *controllers.ScheduleController) {
	schedule := api.Group("/schedule")
	schedule.GET("/export/ics", scheduleController.ExportICS)
	schedule.GET("", scheduleController.List)
	schedule.POST("", scheduleController.Create)
	schedule.GET("/:id", scheduleController.Get)
	schedule.PUT("/:id", scheduleController.Update)
	schedule.DELETE("/:id", scheduleController.Delete)
}
