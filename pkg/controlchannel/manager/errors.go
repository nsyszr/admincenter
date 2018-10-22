package manager

import "errors"

var ErrInvalidRealm = errors.New("oam.controlchannel.manager: invalid realm")
var ErrNoSuchRealm = errors.New("oam.controlchannel.manager: no such realm")
var ErrNoSuchSession = errors.New("oam.controlchannel.manager: no such session")
