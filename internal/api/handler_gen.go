// Code generated by internal/gen/api/main.go. DO NOT EDIT.

package api

import (
	"io"
	"net/http"
)

func (s *Handler) handleBatch(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeBatch(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeBatch(s.api.Batch(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodePublish(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodePublish(s.api.Publish(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeBroadcast(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeBroadcast(s.api.Broadcast(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeSubscribe(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeSubscribe(s.api.Subscribe(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeUnsubscribe(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeUnsubscribe(s.api.Unsubscribe(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeDisconnect(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeDisconnect(s.api.Disconnect(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handlePresence(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodePresence(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodePresence(s.api.Presence(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handlePresenceStats(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodePresenceStats(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodePresenceStats(s.api.PresenceStats(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeHistory(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeHistory(s.api.History(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleHistoryRemove(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeHistoryRemove(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeHistoryRemove(s.api.HistoryRemove(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleInfo(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeInfo(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeInfo(s.api.Info(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleRPC(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeRPC(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeRPC(s.api.RPC(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeRefresh(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeRefresh(s.api.Refresh(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}

func (s *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.handleReadDataError(r, w, err)
		return
	}

	req, err := paramsDecoder.DecodeChannels(data)
	if err != nil {
		s.handleUnmarshalError(r, w, err)
		return
	}

	data, err = responseEncoder.EncodeChannels(s.api.Channels(r.Context(), req))
	if err != nil {
		s.handleMarshalError(r, w, err)
		return
	}

	s.writeJson(w, data)
}