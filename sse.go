// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type managedAgentSSEEvent struct {
	Event string
	ID    string
	Retry *int
	Data  string
}

func renderManagedAgentSSE(writer io.Writer, reader io.Reader) error {
	if writer == nil {
		return fmt.Errorf("response writer is required")
	}
	if reader == nil {
		return fmt.Errorf("event stream reader is required")
	}

	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	var event managedAgentSSEEvent
	var data bytes.Buffer
	dispatch := func() error {
		if data.Len() == 0 && event.Event == "" && event.ID == "" && event.Retry == nil {
			return nil
		}
		event.Data = strings.TrimSuffix(data.String(), "\n")
		if err := writeManagedAgentSSEEvent(writer, event); err != nil {
			return err
		}
		event = managedAgentSSEEvent{}
		data.Reset()
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if ok && strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		if !ok {
			field = line
			value = ""
		}
		switch field {
		case "event":
			event.Event = value
		case "id":
			event.ID = value
		case "retry":
			retry, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid SSE retry value %q: %w", value, err)
			}
			event.Retry = &retry
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func writeManagedAgentSSEEvent(writer io.Writer, event managedAgentSSEEvent) error {
	var data interface{} = event.Data
	trimmedData := strings.TrimSpace(event.Data)
	if trimmedData != "" {
		var decoded interface{}
		if err := json.Unmarshal([]byte(trimmedData), &decoded); err == nil {
			data = decoded
		}
	}

	var out interface{}
	if event.Event == "" && event.ID == "" && event.Retry == nil {
		out = data
	} else {
		wrapped := map[string]interface{}{"data": data}
		if event.Event != "" {
			wrapped["event"] = event.Event
		}
		if event.ID != "" {
			wrapped["id"] = event.ID
		}
		if event.Retry != nil {
			wrapped["retry"] = *event.Retry
		}
		out = wrapped
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = writer.Write(append(encoded, '\n'))
	return err
}
