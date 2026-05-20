package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"net/http"
)

func SetupCollectiveInsightRoutes(controller *controllers.CollectiveInsightController, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userSecret, f)
	}

	http.HandleFunc("/api/collective-insights/recommendations", userAuth(controller.Route))
	http.HandleFunc("/api/collective-insights/top-companies", userAuth(controller.Route))
	http.HandleFunc("/api/collective-insights/consent", userAuth(controller.Route))
	http.HandleFunc("/api/collective-insights/actions", userAuth(controller.Route))
}
