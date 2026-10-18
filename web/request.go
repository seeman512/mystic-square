package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

func parseGameForm(w http.ResponseWriter, r *http.Request) error {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	}
	return r.ParseForm()
}

func parsePositiveID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid positive id")
	}
	return id, nil
}

func parseCoordinate(value string) (int, error) {
	coordinate, err := strconv.Atoi(value)
	if err != nil || coordinate < 0 {
		return 0, errors.New("invalid coordinate")
	}
	return coordinate, nil
}

func queryID(r *http.Request, key string) (uint64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	return parsePositiveID(value)
}

func gameIDFromPath(path string) (uint64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "game" || parts[2] != "move" {
		return 0, errors.New("invalid game route")
	}
	return parsePositiveID(parts[1])
}

func isJSONRequest(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, value any) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthBodySize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func parseForm(w http.ResponseWriter, r *http.Request) (mapValues, error) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return mapValues(r.Form), nil
}

// mapValues keeps the handler independent of the mutable Request.Form map and
// exposes only the read operation needed by the authentication forms.
type mapValues map[string][]string

func (values mapValues) Get(key string) string {
	items := values[key]
	if len(items) == 0 {
		return ""
	}
	return items[0]
}
