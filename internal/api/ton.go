package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/openbuilders/batch-sender/internal/types"
)

func (s *Server) SendHandler(w http.ResponseWriter, r *http.Request) (
	interface{}, error) {
	s.log.Info("Accepted a new transaction")

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.log.Error("Unable to read request body", "error", err)
		return nil, err
	}
	defer r.Body.Close()

	var message types.SendTONMessage

	err = json.Unmarshal(bodyBytes, &message)
	if err != nil {
		return nil, fmt.Errorf("message unmarshalling error: %w", err)
	}

	s.log.Debug("Message", "data", message)

	err = s.publisher.Publish(bodyBytes)
	if err != nil {
		s.log.Error(
			"couldn't enqueue message",
			"message", message,
			"error", err,
		)

		return nil, &APIError{EnqueueingError}
	}

	return "ok", nil
}
