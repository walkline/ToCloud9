package packet

// NewDestroyObjectPacket builds the 3.3.5a SMSG_DESTROY_OBJECT payload.
// Unlike many object-update fields, this opcode uses an unpacked uint64 GUID.
func NewDestroyObjectPacket(objectGUID uint64, onDeath bool) *Packet {
	w := NewWriterWithSize(SMsgDestroyObject, 9)
	w.Uint64(objectGUID).Bool(onDeath)
	return w.ToPacket()
}
