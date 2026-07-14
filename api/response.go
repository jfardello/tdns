package api

import (
	"encoding/json"
	"net/http"

	"github.com/jfardello/tdns/log"
)

func writeJSON(res Response, w http.ResponseWriter) {
	logger := log.GetLogger("api", "writeJSON")
	encoded, err := json.Marshal(res)
	if err != nil {
		logger.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(encoded)
	if err != nil {
		logger.Fatal(err)
	}

}

func formatBool(status bool) string {
	if status {
		return "enabled"
	} else {
		return "disabled"
	}
}
