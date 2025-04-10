package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/openbuilders/batch-sender/internal/types"
)

func (s *Server) JobHandler(w http.ResponseWriter, r *http.Request) (
	interface{}, error) {
	s.log.Info("Accepted a new job")
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.log.Error("Unable to read request body", "error", err)
		return nil, err
	}
	defer r.Body.Close()

	var transaction types.Transaction

	err = json.Unmarshal(bodyBytes, &transaction)
	if err != nil {
		return nil, fmt.Errorf("update unmarshalling error: %w", err)
	}

	s.log.Info("Transaction", "data", transaction)
	return "ok", nil
}
