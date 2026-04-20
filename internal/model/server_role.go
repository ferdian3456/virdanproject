package model

import (
	"github.com/bytedance/sonic"
)

const OwnerRole = "Owner"
const MemberRole = "Member"

type ServerRole struct {
	Id          string
	ServerId    string
	Name        string
	Permissions sonic.NoCopyRawMessage
	Audit
}
