package protocol

import (
	"crypto/rand"
	"math/big"
)

// idAlphabet matches the ID contract regex [2-9A-HJK-NP-Zb-hj-np-z]:
// digits minus 0/1, A-Z minus I/O, a-z minus a/i/o (55 chars).
const idAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZbcdefghjkmnpqrstuvwxyz"

// idMax is the per-char range for idWithPrefix, hoisted to package level:
// ids are minted per streamed event, so this removes one big.NewInt
// allocation per mint (the per-char rand.Int allocations remain — see
// BenchmarkNewID for the measured baseline).
var idMax = big.NewInt(int64(len(idAlphabet)))

func NewID(prefix string) string { return idWithPrefix(prefix) }

func NewEventID() string { return idWithPrefix("evt") }

func idWithPrefix(prefix string) string {
	out := make([]byte, 20)
	for i := range out {
		n, err := rand.Int(rand.Reader, idMax)
		if err != nil {
			panic(err)
		}
		out[i] = idAlphabet[n.Int64()]
	}
	return prefix + "_" + string(out)
}
