package ue

import (
	"fmt"
	"regexp"

	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
	"github.com/free5gc/util/milenage"
	"github.com/free5gc/util/ueauth"
)

// Key derivations copied from free-ran-ue ue/security.go (TS 33.501
// Annex A), reshaped to return values instead of mutating a UE.

var reSupiDigits = regexp.MustCompile(`(?:imsi|supi)-([0-9]{5,15})`)

func deriveKAmf(supi string, ckik []byte, snName string, sqn, ak []byte) ([]byte, error) {
	sqnXorAk := make([]byte, 6)
	for i := range sqnXorAk {
		sqnXorAk[i] = sqn[i] ^ ak[i]
	}
	kausf, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_KAUSF_DERIVATION,
		[]byte(snName), ueauth.KDFLen([]byte(snName)), sqnXorAk, ueauth.KDFLen(sqnXorAk))
	if err != nil {
		return nil, fmt.Errorf("derive kausf: %w", err)
	}
	kseaf, err := ueauth.GetKDFValue(kausf, ueauth.FC_FOR_KSEAF_DERIVATION,
		[]byte(snName), ueauth.KDFLen([]byte(snName)))
	if err != nil {
		return nil, fmt.Errorf("derive kseaf: %w", err)
	}
	groups := reSupiDigits.FindStringSubmatch(supi)
	if groups == nil {
		return nil, fmt.Errorf("supi %q has no imsi digits", supi)
	}
	p0, p1 := []byte(groups[1]), []byte{0x00, 0x00} // ABBA = 0x0000
	return ueauth.GetKDFValue(kseaf, ueauth.FC_FOR_KAMF_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
}

// deriveNasKeys returns the 128-bit KNASenc and KNASint for the given
// algorithms (the low 16 bytes of each 256-bit KDF output).
func deriveNasKeys(kAmf []byte, enc ie.AlgCiphering, integ ie.AlgIntegrity) (kenc, kint [16]byte, err error) {
	p0, p1 := []byte{message.NNASEncAlg}, []byte{byte(enc)}
	e, err := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	if err != nil {
		return kenc, kint, fmt.Errorf("derive knas_enc: %w", err)
	}
	p0, p1 = []byte{message.NNASIntAlg}, []byte{byte(integ)}
	i, err := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	if err != nil {
		return kenc, kint, fmt.Errorf("derive knas_int: %w", err)
	}
	copy(kenc[:], e[16:32])
	copy(kint[:], i[16:32])
	return kenc, kint, nil
}

// akaResult is what the UE learns from a successful 5G-AKA challenge.
type akaResult struct {
	resStar []byte
	kAmf    []byte
}

// runAKA verifies AUTN (MAC check inside milenage) and derives RES* and
// KAMF. The SQN in AUTN is accepted as-is: like free-ran-ue, the tester
// does not keep SQN state, so re-running the same UEs never needs a resync.
func runAKA(supi string, k, opc, rand, autn []byte, snName string) (akaResult, error) {
	sqn, ak, ik, ck, res, err := milenage.GenerateKeysWithAUTN(opc, k, rand, autn)
	if err != nil {
		return akaResult{}, fmt.Errorf("verify autn: %w", err)
	}
	ckik := append(append([]byte{}, ck...), ik...)
	kAmf, err := deriveKAmf(supi, ckik, snName, sqn, ak)
	if err != nil {
		return akaResult{}, err
	}
	p0 := []byte(snName)
	out, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION,
		p0, ueauth.KDFLen(p0), rand, ueauth.KDFLen(rand), res, ueauth.KDFLen(res))
	if err != nil {
		return akaResult{}, fmt.Errorf("derive res*: %w", err)
	}
	return akaResult{resStar: out[len(out)/2:], kAmf: kAmf}, nil
}

// servingNetworkName is "5G:mncXXX.mccYYY.3gppnetwork.org" (TS 24.501).
func servingNetworkName(mcc, mnc string) string {
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}
	return fmt.Sprintf("5G:mnc%s.mcc%s.3gppnetwork.org", mnc, mcc)
}
