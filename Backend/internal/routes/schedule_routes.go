package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupScheduleRoutes(api *echo.Group, scheduleController *controllers.ScheduleController) {
	schedule := api.Group("/schedule")
	schedule.Any("/export/ics", wrap(scheduleController.ExportICS))
	schedule.Any("", wrap(scheduleController.RouteList))
	schedule.Any("/:id", wrap(scheduleController.RouteByID))
}
