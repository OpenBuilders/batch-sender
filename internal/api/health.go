package api

import (
	"net/http"
)

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	/*status := s.healthChecker.GetHealthStatus()

	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		return status, nil
	}

	return status, nil*/
	return "ok", nil
}

func (s *Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	/*status := s.healthChecker.GetHealthStatus()

	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		return false, nil
	}

	if !s.catalog.IsReady() || !s.warehouse.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return false, nil
	}*/

	return true, nil
}
