package sitemanager

import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/time"
)

type RequestStatus uint8

const (
	RequestPending RequestStatus = iota // ZERO VALUE
	RequestAccepted
	RequestRejected
)

func (s RequestStatus) String() string {
	switch s {
	case RequestPending:
		return "pending"
	case RequestAccepted:
		return "accepted"
	case RequestRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// Request registers an access request. If a pending request already exists for that email,
// returns the existing one and created=false.
func (m *Module) Request(email, name, message string) (req AccessRequest, created bool, err error) {
	if email == "" {
		return AccessRequest{}, false, ErrInvalidData
	}

	pending, err := ReadAllAccessRequest(
		m.db.Query(&AccessRequest{}).
			Where(AccessRequest_.Email).Eq(email).
			Where(AccessRequest_.Status).Eq(int64(RequestPending)),
	)
	if err == nil && len(pending) > 0 {
		return *pending[0], false, nil
	}

	nowSec := time.Now() / 1_000_000_000
	newID := m.ids.NewID()
	req = AccessRequest{
		Id:        newID,
		Email:     email,
		Name:      name,
		Message:   message,
		CreatedAt: nowSec,
		Status:    int64(RequestPending),
	}

	if err := m.db.Create(&req); err != nil {
		return AccessRequest{}, false, err
	}

	return req, true, nil
}

// AcceptRequest accepts an access request by setting its status to RequestAccepted.
// Note: Accepting an AccessRequest creates ZERO SiteMember entries.
func (m *Module) AcceptRequest(id string) error {
	var req AccessRequest
	err := m.db.Query(&req).Where(AccessRequest_.Id).Eq(id).ReadOne()
	if err != nil {
		return err
	}
	req.Status = int64(RequestAccepted)
	return m.db.Update(&req, orm.Eq(AccessRequest_.Id, id))
}

// PendingRequests devuelve las solicitudes de acceso sin resolver, de la más
// antigua a la más reciente.
// Nota: Deja el orden natural de inserción que en mem/D1 coincide con el de creación.
func (m *Module) PendingRequests() ([]AccessRequest, error) {
	rows, err := ReadAllAccessRequest(
		m.db.Query(&AccessRequest{}).
			Where(AccessRequest_.Status).Eq(int64(RequestPending)),
	)
	if err != nil {
		return nil, err
	}
	out := make([]AccessRequest, len(rows))
	for i := range rows {
		out[i] = *rows[i]
	}
	return out, nil
}

// RequestByID devuelve la solicitud de acceso con ese id.
func (m *Module) RequestByID(id string) (AccessRequest, error) {
	var req AccessRequest
	err := m.db.Query(&req).Where(AccessRequest_.Id).Eq(id).ReadOne()
	if err != nil {
		return AccessRequest{}, err
	}
	return req, nil
}
