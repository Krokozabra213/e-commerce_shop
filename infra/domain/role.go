package infradomain

type Role string

const (
	RoleUser    Role = "ROLE_USER"
	RoleManager Role = "ROLE_MANAGER"
	RoleAdmin   Role = "ROLE_ADMIN"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleUser, RoleManager, RoleAdmin:
		return true
	}
	return false
}

func (r Role) Weight() int {
	switch r {
	case RoleUser:
		return 1
	case RoleManager:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}

func (r Role) String() string {
	return string(r)
}
