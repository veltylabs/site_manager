package sitemanager

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
