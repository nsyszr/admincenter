package device

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/ory/herodot"
	"github.com/pkg/errors"
)

type Handler struct {
	Manager Manager
	W       herodot.Writer
}

const (
	DevicesHandlerPath = "/devices"
)

func NewHandler(manager Manager, w herodot.Writer) *Handler {
	return &Handler{
		Manager: manager,
		W:       w,
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	// withCORS := middleware.CORSHandler()

	r.HandleFunc(DevicesHandlerPath, h.List).Methods("GET", "OPTIONS")
	r.HandleFunc(DevicesHandlerPath, h.Create).Methods("POST", "OPTIONS")
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	//limit, offset := pagination.Parse(r, 100, 0, 500)
	var limit, offset = 100, 0
	devices, err := h.Manager.GetDevices(r.Context(), limit, offset)
	if err != nil {
		h.W.WriteError(w, r, err)
		return
	}

	res := make([]Device, len(devices))
	i := 0
	for _, device := range devices {
		res[i] = device
		i++
	}

	h.W.Write(w, r, res)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var device Device

	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.W.WriteError(w, r, errors.WithStack(err))
		return
	}

	/*if err := h.Validator.Validate(&c); err != nil {
		h.H.WriteError(w, r, err)
		return
	}*/

	// Generate a UUID for device ID
	device.DeviceID = uuid.New().String()

	// Set creation and update date
	device.CreatedAt = time.Now().UTC().Round(time.Second)
	device.UpdatedAt = device.CreatedAt

	if err := h.Manager.CreateDevice(r.Context(), &device); err != nil {
		h.W.WriteError(w, r, err)
		return
	}

	h.W.WriteCreated(w, r, DevicesHandlerPath+"/"+device.GetID(), &device)
}
