package awdl

type APlist struct {
	ReceiverComputerName             string
	ReceiverModelName                string
	ReceiverMediaCapabilitiesBytes   []byte
	ReceiverMediaCapabilitiesRecords ReceiverMediaCapabilities
}

// Got this by converting json to struct: https://mholt.github.io/json-to-go/
type ReceiverMediaCapabilities struct {
	SupportsAdjustmentBaseResources bool `json:"SupportsAdjustmentBaseResources"`
	Version                         int  `json:"Version"`
	Codecs                          struct {
		Hvc1 struct {
			Profiles struct {
				VTIsHDRAllowedOnDevice      bool     `json:"VTIsHDRAllowedOnDevice"`
				VTSupportedProfiles         []int    `json:"VTSupportedProfiles"`
				VTDoViIsHardwareAccelerated bool     `json:"VTDoViIsHardwareAccelerated"`
				VTDoViSupportedLevels       []string `json:"VTDoViSupportedLevels"`
				VTPerProfileSupport         struct {
					Num1 struct {
						VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
						VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
					} `json:"1"`
					Num2 struct {
						VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
						VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
					} `json:"2"`
					Num4 struct {
						VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
						VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
					} `json:"4"`
				} `json:"VTPerProfileSupport"`
				VTDoViSupportedProfiles []string `json:"VTDoViSupportedProfiles"`
			} `json:"Profiles"`
		} `json:"hvc1"`
		CodecSupport struct {
			VTIsHDRAllowedOnDevice bool `json:"VTIsHDRAllowedOnDevice"`
			VTCodecSupportDict     struct {
				Apcs struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"apcs"`
				Hvc1 struct {
					VTPerProfileSupport struct {
						Num1 struct {
							VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
							VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
						} `json:"1"`
						Num2 struct {
							VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
							VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
						} `json:"2"`
						Num4 struct {
							VTMaxDecodeLevel        int  `json:"VTMaxDecodeLevel"`
							VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
						} `json:"4"`
					} `json:"VTPerProfileSupport"`
					VTSupportedProfiles []int `json:"VTSupportedProfiles"`
				} `json:"hvc1"`
				Apcn struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"apcn"`
				Apch struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"apch"`
				Apco struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"apco"`
				Ap4X struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"ap4x"`
				Ap4H struct {
					VTIsHardwareAccelerated bool `json:"VTIsHardwareAccelerated"`
				} `json:"ap4h"`
			} `json:"VTCodecSupportDict"`
		} `json:"CodecSupport"`
	} `json:"Codecs"`
	ContainerFormats struct {
		PublicHeifStandard struct {
			HeifSubtypes []string `json:"HeifSubtypes"`
		} `json:"public.heif-standard"`
	} `json:"ContainerFormats"`
	Vendor struct {
		ComApple struct {
			LivePhotoFormatVersion   string `json:"LivePhotoFormatVersion"`
			AssetBundleFormatVersion string `json:"AssetBundleFormatVersion"`
		} `json:"com.apple"`
	} `json:"Vendor"`
}

// Sample JSON from plist file:
/*
{
	"SupportsAdjustmentBaseResources": true,
	"Version": 3,
	"Codecs": {
		"hvc1": {
			"Profiles": {
				"VTIsHDRAllowedOnDevice": true,
				"VTSupportedProfiles": [1, 2, 4],
				"VTDoViIsHardwareAccelerated": true,
				"VTDoViSupportedLevels": ["01", "02", "03", "04", "05", "06", "07"],
				"VTPerProfileSupport": {
					"1": {
						"VTMaxDecodeLevel": 180,
						"VTIsHardwareAccelerated": true
					},
					"2": {
						"VTMaxDecodeLevel": 180,
						"VTIsHardwareAccelerated": true
					},
					"4": {
						"VTMaxDecodeLevel": 180,
						"VTIsHardwareAccelerated": true
					}
				},
				"VTDoViSupportedProfiles": ["05"]
			}
		},
		"CodecSupport": {
			"VTIsHDRAllowedOnDevice": true,
			"VTCodecSupportDict": {
				"apcs": {
					"VTIsHardwareAccelerated": true
				},
				"hvc1": {
					"VTPerProfileSupport": {
						"1": {
							"VTMaxDecodeLevel": 180,
							"VTIsHardwareAccelerated": true
						},
						"2": {
							"VTMaxDecodeLevel": 180,
							"VTIsHardwareAccelerated": true
						},
						"4": {
							"VTMaxDecodeLevel": 180,
							"VTIsHardwareAccelerated": true
						}
					},
					"VTSupportedProfiles": [1, 2, 4]
				},
				"apcn": {
					"VTIsHardwareAccelerated": true
				},
				"apch": {
					"VTIsHardwareAccelerated": true
				},
				"apco": {
					"VTIsHardwareAccelerated": true
				},
				"ap4x": {
					"VTIsHardwareAccelerated": true
				},
				"ap4h": {
					"VTIsHardwareAccelerated": true
				}
			}
		}
	},
	"ContainerFormats": {
		"public.heif-standard": {
			"HeifSubtypes": ["public.avci", "public.avif", "public.heic", "public.heics", "public.heif"]
		}
	},
	"Vendor": {
		"com.apple": {
			"LivePhotoFormatVersion": "1",
			"AssetBundleFormatVersion": "1"
		}
	}
}
*/
