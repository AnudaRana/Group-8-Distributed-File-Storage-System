package api

import (
	"io/ioutil"
	"net/http"

	"dfs-system/internal/config"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var cfg = config.LoadConfig()

func MessageHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	msg, err := types.ParseMessage(body)
	if err != nil {
		http.Error(w, "Invalid message", 400)
		return
	}
	utils.Log(cfg.NodeID, "Received [%s] message from node: %s", msg.Type, msg.Sender)
	w.WriteHeader(http.StatusOK)
}