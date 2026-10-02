package gnb

import (
	"encoding/hex"
	"fmt"

	"github.com/free-ran-ue/util"
	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/free5gc/openapi/models"

	"tester/profile"
)

// Identity is the NGAP-encoded form of one gNB's settings.
type Identity struct {
	GnbID  []byte
	Name   string
	PlmnID ie.PLMNIdentity
	Tai    ie.TAI
	Snssai ie.SNSSAI
}

// NewIdentity encodes spec + tpl. The profile package has already
// validated the formats; errors here mean a caller skipped profile.Expand.
func NewIdentity(spec profile.GnbSpec, tpl profile.GnbTemplate) (Identity, error) {
	gnbID, err := hex.DecodeString(spec.GnbID)
	if err != nil {
		return Identity{}, fmt.Errorf("gnb id %q: %w", spec.GnbID, err)
	}
	plmn := models.PlmnId{Mcc: tpl.Mcc, Mnc: tpl.Mnc}
	plmnID, err := util.PlmnIdToNgap(plmn)
	if err != nil {
		return Identity{}, fmt.Errorf("plmn %s-%s: %w", tpl.Mcc, tpl.Mnc, err)
	}
	tai, err := util.TaiToNgap(models.Tai{PlmnId: &plmn, Tac: tpl.Tac})
	if err != nil {
		return Identity{}, fmt.Errorf("tac %q: %w", tpl.Tac, err)
	}
	snssai, err := util.SNssaiToNgap(models.Snssai{Sst: int32(tpl.Sst), Sd: tpl.Sd})
	if err != nil {
		return Identity{}, fmt.Errorf("snssai %d-%s: %w", tpl.Sst, tpl.Sd, err)
	}
	return Identity{GnbID: gnbID, Name: spec.Name, PlmnID: plmnID, Tai: tai, Snssai: snssai}, nil
}

// NGSetupRequest encodes the NG Setup Request for this gNB, matching the
// one free-ran-ue sends (gnb/ngapBuilder.go buildNgapSetupRequest).
func (id Identity) NGSetupRequest() ([]byte, error) {
	plmnID, snssai := id.PlmnID, id.Snssai
	req := &message.NGSetupRequest{
		GlobalRANNodeID: &ie.GlobalRANNodeID{
			Choice: &ie.GlobalGNBID{
				PLMNIdentity: &plmnID,
				GNBID: &ie.GNBID{
					Choice: &ie.GNBIDForGNBID{
						Value: aper.BitString{Bytes: id.GnbID, BitLength: uint64(len(id.GnbID) * 8)},
					},
				},
			},
		},
		RANNodeName: &ie.RANNodeName{Value: aper.PrintableString(id.Name)},
		SupportedTAList: &ie.SupportedTAList{
			List: []ie.SupportedTAItem{{
				TAC: id.Tai.TAC,
				BroadcastPLMNList: &ie.BroadcastPLMNList{
					List: []ie.BroadcastPLMNItem{{
						PLMNIdentity:        id.Tai.PLMNIdentity,
						TAISliceSupportList: &ie.SliceSupportList{List: []ie.SliceSupportItem{{SNSSAI: &snssai}}},
					}},
				},
			}},
		},
		DefaultPagingDRX: &ie.PagingDRX{Value: ie.PagingDRXPresentV128},
	}
	return req.MarshalBinary()
}
