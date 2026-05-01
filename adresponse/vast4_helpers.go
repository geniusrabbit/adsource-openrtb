package adresponse

import (
	"encoding/xml"

	"github.com/haxqer/vast"
	"github.com/pkg/errors"
)

var (
	errMultipleAdsNotSupported       = errors.Wrap(ErrUnsupportedVASTConfiguration, "multiple ads are not supported")
	errMultipleCreativesNotSupported = errors.Wrap(ErrUnsupportedVASTConfiguration, "multiple creatives are not supported")
	errConditionalAdNotSupported     = errors.Wrap(ErrUnsupportedVASTConfiguration, "conditional ad is not supported")
	errEmptyVASTAdTagURI             = errors.Wrap(ErrUnsupportedVASTConfiguration, "empty VASTAdTagURI in wrapper")
	errInvalidInlineAdConfig         = errors.Wrap(ErrUnsupportedVASTConfiguration, "invalid inline ad configuration")
	errInvalidWrapperAdConfig        = errors.Wrap(ErrUnsupportedVASTConfiguration, "invalid wrapper ad configuration")
	errUnsupportedAdConfig           = errors.Wrap(ErrUnsupportedVASTConfiguration, "unsupported ad configuration")
)

func unmarshalVAST(data []byte) (*vast.VAST, error) {
	var v vast.VAST
	err := xml.Unmarshal(data, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func validateVAST(v *vast.VAST) error {
	if v == nil || len(v.Ads) == 0 {
		return ErrInvalidVAST
	}
	if len(v.Ads) > 1 {
		return errMultipleAdsNotSupported
	}
	if v.Ads[0].ConditionalAd {
		return errConditionalAdNotSupported
	}
	for _, ad := range v.Ads {
		if ad.Wrapper != nil {
			switch {
			case ad.Wrapper.VASTAdTagURI.CDATA == "":
				return errEmptyVASTAdTagURI
			case len(ad.Wrapper.Creatives) == 0:
				return errInvalidWrapperAdConfig
			case len(ad.Wrapper.Creatives) > 1:
				return errMultipleCreativesNotSupported
			default:
				if err := validateVASTCreativeWrapper(&ad.Wrapper.Creatives[0]); err != nil {
					return err
				}
			}
		} else if ad.InLine != nil {
			switch {
			case ad.InLine.AdTitle.CDATA == "" || len(ad.InLine.Creatives) == 0:
				return errInvalidInlineAdConfig
			case len(ad.InLine.Creatives) == 1:
				return errInvalidInlineAdConfig
			case len(ad.InLine.Creatives) > 1:
				return errMultipleCreativesNotSupported
			default:
				if err := validateVASTCreative(&ad.InLine.Creatives[0]); err != nil {
					return err
				}
			}
		} else {
			return errUnsupportedAdConfig
		}
	}
	return nil
}

func validateVASTCreativeWrapper(v *vast.CreativeWrapper) error {
	if v.Linear == nil {
		return errInvalidWrapperAdConfig
	} else if v.Linear != nil {
		if v.Linear.VideoClicks == nil || len(v.Linear.VideoClicks.ClickThroughs) == 0 {
			return errors.Wrap(errInvalidWrapperAdConfig, "missing video clicks or click-throughs")
		}
	} else if v.CompanionAds != nil && len(v.CompanionAds.Companions) > 0 {
		return errors.Wrap(errUnsupportedAdConfig, "companion ads are not supported")
	} else if v.NonLinearAds != nil && len(v.NonLinearAds.NonLinears) > 0 {
		return errors.Wrap(errUnsupportedAdConfig, "non-linear ads are not supported")
	} else {
		return errUnsupportedAdConfig
	}
	return nil
}

func validateVASTCreative(creative *vast.Creative) error {
	if creative.Linear == nil {
		return errInvalidInlineAdConfig
	} else if creative.Linear != nil {
		if creative.Linear.VideoClicks == nil || len(creative.Linear.VideoClicks.ClickThroughs) == 0 {
			return errors.Wrap(errInvalidInlineAdConfig, "missing video clicks or click-throughs")
		}
	} else if creative.CompanionAds != nil && len(creative.CompanionAds.Companions) > 0 {
		return errors.Wrap(errUnsupportedAdConfig, "companion ads are not supported")
	} else if creative.NonLinearAds != nil && len(creative.NonLinearAds.NonLinears) > 0 {
		return errors.Wrap(errUnsupportedAdConfig, "non-linear ads are not supported")
	} else {
		return errUnsupportedAdConfig
	}
	return nil
}
