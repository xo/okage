// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"slices"
	"strings"

	"github.com/xo/okage/internal/xmpwriter"
)

// This file ports krilla src/interchange/metadata/predefined.rs (D99).
// Upstream builds the managed schemas and the predefined structure types
// once, in a LazyLock. The port builds them when the package starts.

// managedSchemaList holds the schemas of managedSchemas.
var managedSchemaList = buildManagedSchemas()

// managedSchemas is managed_schemas in krilla
// src/interchange/metadata/predefined.rs. Do not change the schemas.
//
// Schemas describing the properties krilla writes itself, either from the
// Metadata or based on the configuration.
func managedSchemas() []Schema {
	return managedSchemaList
}

// managedProperty is a property of a managed schema: its name, its value
// type and its description.
type managedProperty struct {
	name        string
	valueType   IntoValueType
	description string
}

// buildManagedSchemas builds the schemas of managed_schemas in krilla
// src/interchange/metadata/predefined.rs.
func buildManagedSchemas() []Schema {
	schema := func(ns xmpwriter.Namespace, properties []managedProperty) Schema {
		s := NewSchema(NewNamespace(ns.Prefix(), ns.URL())).Name(ns.Name())
		for _, p := range properties {
			s = s.Property(NewPropertyDescription(p.name, p.valueType, p.description))
		}
		return s
	}

	return []Schema{
		schema(xmpwriter.NamespaceDublinCore, []managedProperty{
			{"title", LanguageAlternativeType(), "The title of the document"},
			{"description", LanguageAlternativeType(), "A textual description of the content of the document"},
			{"creator", OrderedArrayType(SimpleTypeProperName), "The authors of the document"},
			{"language", UnorderedArrayType(SimpleTypeLocale), "The languages used in the document"},
			{"format", SimpleTypeMIMEType, "The file format of the document"},
		}),
		schema(xmpwriter.NamespaceXmp, []managedProperty{
			{"CreatorTool", SimpleTypeAgentName, "The name of the first known tool used to create the document"},
			{"CreateDate", SimpleTypeDate, "The date the document was created"},
			{"ModifyDate", SimpleTypeDate, "The date the document was last modified"},
		}),
		schema(xmpwriter.NamespaceXmpMedia, []managedProperty{
			{"InstanceID", SimpleTypeGUID, "An identifier for this incarnation of the document"},
			{"DocumentID", SimpleTypeGUID, "The common identifier for all versions of the document"},
			{"VersionID", SimpleTypeText, "The version identifier of the document"},
			{"RenditionClass", SimpleTypeRenditionClass, "The rendition class of the document"},
			{"History", OrderedArrayType(PredefinedStructResourceEvent), "The high-level actions that resulted in this document"},
		}),
		schema(xmpwriter.NamespaceXmpPaged, []managedProperty{
			{"NPages", SimpleTypeInteger, "The number of pages in the document"},
		}),
		schema(xmpwriter.NamespaceAdobePdf, []managedProperty{
			{"Keywords", SimpleTypeText, "Keywords associated with the document"},
			{"PDFVersion", SimpleTypeText, "The PDF version of the document"},
			{"Producer", SimpleTypeAgentName, "The name of the tool that created the PDF"},
			{"Trapped", SimpleTypeBoolean, "Whether the document has been trapped"},
		}),
		schema(xmpwriter.NamespacePdfAID, []managedProperty{
			{"part", SimpleTypeInteger, "The part of ISO 19005 the document conforms to"},
			{"rev", SimpleTypeInteger, "The year of the revision of the part"},
			{"conformance", SimpleTypeText, "The conformance level of the part"},
			{"amd", SimpleTypeText, "The amendment identifier"},
			{"corr", SimpleTypeText, "The corrigendum identifier"},
		}),
		schema(xmpwriter.NamespacePdfUAID, []managedProperty{
			{"part", SimpleTypeInteger, "The part of ISO 14289 the document conforms to"},
			{"rev", SimpleTypeInteger, "The year of the revision of the part"},
			{"amd", SimpleTypeText, "The amendment identifier"},
			{"corr", SimpleTypeText, "The corrigendum identifier"},
		}),
	}
}

// managedPrefix is managed_prefix in krilla
// src/interchange/metadata/predefined.rs. It returns false where upstream
// returns None.
//
// The prefix of the managed property name in uri, if it is one.
func managedPrefix(uri, name string) (string, bool) {
	for i := range managedSchemaList {
		s := &managedSchemaList[i]
		if _, ok := s.get(name); ok && s.namespace.uri == uri {
			return s.namespace.prefix, true
		}
	}
	for _, ns := range []xmpwriter.Namespace{
		xmpwriter.NamespacePdfAExtension,
		xmpwriter.NamespacePdfASchema,
		xmpwriter.NamespacePdfAProperty,
		xmpwriter.NamespacePdfAType,
		xmpwriter.NamespacePdfAField,
	} {
		if ns.URL() == uri {
			return ns.Prefix(), true
		}
	}
	return "", false
}

// predefinedProperties is an item of the lists of predefined properties in
// krilla src/interchange/metadata/predefined.rs: a namespace URI and the
// sorted names of its properties.
type predefinedProperties struct {
	uri   string
	names []string
}

// pdfA1PredefinedProperties is PDF_A1_PREDEFINED_PROPERTIES in krilla
// src/interchange/metadata/predefined.rs.
//
// The properties PDF/A-1 predefines: those of the schemas in the XMP
// Specification, January 2004, plus the PDF/A identification schema of ISO
// 19005-1.
//
// Sorted by namespace URI and property name.
var pdfA1PredefinedProperties = []predefinedProperties{
	{"http://ns.adobe.com/exif/1.0/", []string{
		"ApertureValue", "BrightnessValue", "CFAPattern", "ColorSpace",
		"ComponentsConfiguration", "CompressedBitsPerPixel", "Contrast",
		"CustomRendered", "DateTimeDigitized", "DateTimeOriginal",
		"DeviceSettingDescription", "DigitalZoomRatio", "ExifVersion",
		"ExposureBiasValue", "ExposureIndex", "ExposureMode", "ExposureProgram",
		"ExposureTime", "FNumber", "FileSource", "Flash", "FlashEnergy",
		"FlashpixVersion", "FocalLength", "FocalLengthIn35mmFilm",
		"FocalPlaneResolutionUnit", "FocalPlaneXResolution",
		"FocalPlaneYResolution", "GPSAltitude", "GPSAltitudeRef",
		"GPSAreaInformation", "GPSDOP", "GPSDestBearing", "GPSDestBearingRef",
		"GPSDestDistance", "GPSDestDistanceRef", "GPSDestLatitude",
		"GPSDestLongitude", "GPSDifferential", "GPSImgDirection",
		"GPSImgDirectionRef", "GPSLatitude", "GPSLongitude", "GPSMapDatum",
		"GPSMeasureMode", "GPSProcessingMethod", "GPSSatellites", "GPSSpeed",
		"GPSSpeedRef", "GPSStatus", "GPSTimeStamp", "GPSTrack", "GPSTrackRef",
		"GPSVersionID", "GainControl", "ISOSpeedRatings", "ImageUniqueID",
		"LightSource", "MakerNote", "MaxApertureValue", "MeteringMode", "OECF",
		"PixelXDimension", "PixelYDimension", "RelatedSoundFile", "Saturation",
		"SceneCaptureType", "SceneType", "SensingMethod", "Sharpness",
		"ShutterSpeedValue", "SpatialFrequencyResponse", "SpectralSensitivity",
		"SubjectArea", "SubjectDistance", "SubjectDistanceRange",
		"SubjectLocation", "UserComment", "WhiteBalance",
	}},
	{"http://ns.adobe.com/pdf/1.3/", []string{
		"Keywords", "PDFVersion", "Producer",
	}},
	{"http://ns.adobe.com/photoshop/1.0/", []string{
		"AuthorsPosition", "CaptionWriter", "Category", "City", "Country",
		"Credit", "DateCreated", "Headline", "Instructions", "Source", "State",
		"SupplementalCategories", "TransmissionReference", "Urgency",
	}},
	{"http://ns.adobe.com/tiff/1.0/", []string{
		"Artist", "BitsPerSample", "Compression", "Copyright", "DateTime",
		"ImageDescription", "ImageLength", "ImageWidth", "Make", "Model",
		"Orientation", "PhotometricInterpretation", "PlanarConfiguration",
		"PrimaryChromaticities", "ReferenceBlackWhite", "ResolutionUnit",
		"SamplesPerPixel", "Software", "TransferFunction", "WhitePoint",
		"XResolution", "YCbCrCoefficients", "YCbCrPositioning", "YCbCrSubSampling",
		"YResolution",
	}},
	{"http://ns.adobe.com/xap/1.0/", []string{
		"Advisory", "BaseURL", "CreateDate", "CreatorTool", "Identifier",
		"MetadataDate", "ModifyDate", "Nickname", "Thumbnails",
	}},
	{"http://ns.adobe.com/xap/1.0/bj/", []string{
		"JobRef",
	}},
	{"http://ns.adobe.com/xap/1.0/mm/", []string{
		"DerivedFrom", "DocumentID", "History", "LastURL", "ManageTo", "ManageUI",
		"ManagedFrom", "Manager", "ManagerVariant", "RenditionClass",
		"RenditionOf", "RenditionParams", "SaveID", "VersionID", "Versions",
	}},
	{"http://ns.adobe.com/xap/1.0/rights/", []string{
		"Certificate", "Marked", "Owner", "UsageTerms", "WebStatement",
	}},
	{"http://ns.adobe.com/xap/1.0/t/pg/", []string{
		"MaxPageSize", "NPages",
	}},
	{"http://purl.org/dc/elements/1.1/", []string{
		"contributor", "coverage", "creator", "date", "description", "format",
		"identifier", "language", "publisher", "relation", "rights", "source",
		"subject", "title", "type",
	}},
	{"http://www.aiim.org/pdfa/ns/id/", []string{
		"amd", "conformance", "part",
	}},
}

// pdfA2A3AdditionalProperties is PDF_A2_A3_ADDITIONAL_PROPERTIES in krilla
// src/interchange/metadata/predefined.rs.
//
// The properties PDF/A-2 and PDF/A-3 predefine in addition to
// pdfA1PredefinedProperties: those the XMP Specification, September 2005,
// added to the January 2004 edition, plus pdfaid:corr of ISO 19005-2.
//
// Sorted by namespace URI and property name.
var pdfA2A3AdditionalProperties = []predefinedProperties{
	{"http://ns.adobe.com/camera-raw-settings/1.0/", []string{
		"AutoBrightness", "AutoContrast", "AutoExposure", "AutoShadows", "BlueHue",
		"BlueSaturation", "Brightness", "CameraProfile", "ChromaticAberrationB",
		"ChromaticAberrationR", "ColorNoiseReduction", "Contrast", "CropAngle",
		"CropBottom", "CropHeight", "CropLeft", "CropRight", "CropTop",
		"CropUnits", "CropWidth", "Exposure", "GreenHue", "GreenSaturation",
		"HasCrop", "HasSettings", "LuminanceSmoothing", "RawFileName", "RedHue",
		"RedSaturation", "Saturation", "ShadowTint", "Shadows", "Sharpness",
		"Temperature", "Tint", "ToneCurve", "ToneCurveName", "Version",
		"VignetteAmount", "VignetteMidpoint", "WhiteBalance",
	}},
	{"http://ns.adobe.com/exif/1.0/aux/", []string{
		"Lens", "SerialNumber",
	}},
	{"http://ns.adobe.com/xap/1.0/", []string{
		"Label", "Rating",
	}},
	{"http://ns.adobe.com/xap/1.0/mm/", []string{
		"InstanceID",
	}},
	{"http://ns.adobe.com/xap/1.0/t/pg/", []string{
		"Colorants", "Fonts", "PlateNames",
	}},
	{"http://ns.adobe.com/xmp/1.0/DynamicMedia/", []string{
		"absPeakAudioFilePath", "album", "altTapeName", "altTimecode", "artist",
		"audioChannelType", "audioCompressor", "audioModDate", "audioSampleRate",
		"audioSampleType", "beatSpliceParams", "composer", "contributedMedia",
		"copyright", "duration", "engineer", "fileDataRate", "genre", "instrument",
		"introTime", "key", "logComment", "loop", "markers", "metadataModDate",
		"numberOfBeats", "outCue", "projectRef", "pullDown",
		"relativePeakAudioFilePath", "relativeTimestamp", "releaseDate",
		"resampleParams", "scaleType", "scene", "shotDate", "shotLocation",
		"shotName", "speakerPlacement", "startTimecode", "stretchMode", "tapeName",
		"tempo", "timeScaleParams", "timeSignature", "trackNumber",
		"videoAlphaMode", "videoAlphaPremultipleColor",
		"videoAlphaUnityIsTransparent", "videoColorSpace", "videoCompressor",
		"videoFieldOrder", "videoFrameRate", "videoFrameSize", "videoModDate",
		"videoPixelAspectRatio", "videoPixelDepth",
	}},
	{"http://www.aiim.org/pdfa/ns/id/", []string{
		"corr",
	}},
}

// pdfA1OnlyProperties is PDF_A1_ONLY_PROPERTIES in krilla
// src/interchange/metadata/predefined.rs.
//
// The properties of pdfA1PredefinedProperties that the XMP Specification,
// September 2005, no longer contains.
//
// Sorted by namespace URI and property name.
var pdfA1OnlyProperties = []predefinedProperties{
	{"http://ns.adobe.com/exif/1.0/", []string{
		"MakerNote",
	}},
}

// isPredefinedPDFAProperty is is_predefined_pdfa_property in krilla
// src/interchange/metadata/predefined.rs.
//
// Whether PDF/A exempts the property name in uri from an extension schema
// description. xmp2005 selects the PDF/A-2/-3 rules over the PDF/A-1 rules,
// see validators.usesXMP2005PredefinedSchemas.
func isPredefinedPDFAProperty(uri, name string, xmp2005 bool) bool {
	pdfA1 := containsProperty(pdfA1PredefinedProperties, uri, name)
	if xmp2005 {
		return (pdfA1 && !containsProperty(pdfA1OnlyProperties, uri, name)) ||
			containsProperty(pdfA2A3AdditionalProperties, uri, name)
	}
	return pdfA1
}

// containsProperty is contains_property in krilla
// src/interchange/metadata/predefined.rs.
func containsProperty(properties []predefinedProperties, uri, name string) bool {
	i, ok := slices.BinarySearchFunc(properties, uri, func(p predefinedProperties, uri string) int {
		return strings.Compare(p.uri, uri)
	})
	if !ok {
		return false
	}
	_, ok = slices.BinarySearch(properties[i].names, name)
	return ok
}

// predefinedStructTypes holds the types of predefinedStructType, in the
// order of PredefinedStruct.
var predefinedStructTypes = buildPredefinedStructTypes()

// buildPredefinedStructTypes builds the types of the LazyLock TYPES of
// predefined_struct_type in krilla src/interchange/metadata/predefined.rs.
func buildPredefinedStructTypes() [6]StructType {
	type field struct {
		name      string
		valueType IntoValueType
	}
	structType := func(prefix, uri, name string, fields []field) StructType {
		ty := NewStructType(NewNamespace(prefix, uri), name, "")
		for _, f := range fields {
			ty = ty.Field(NewPropertyDescription(f.name, f.valueType, ""))
		}
		return ty
	}
	open := func(ty SimpleType) ValueType { return OpenChoiceType(ty) }

	return [6]StructType{
		structType("stDim", "http://ns.adobe.com/xap/1.0/sType/Dimensions#", "Dimensions", []field{
			{"w", SimpleTypeReal}, {"h", SimpleTypeReal}, {"unit", open(SimpleTypeText)},
		}),
		structType("xmpGImg", "http://ns.adobe.com/xap/1.0/g/img/", "Thumbnail", []field{
			{"height", SimpleTypeInteger},
			{"width", SimpleTypeInteger},
			{"format", open(SimpleTypeText)},
			{"image", SimpleTypeText},
		}),
		structType("stEvt", "http://ns.adobe.com/xap/1.0/sType/ResourceEvent#", "ResourceEvent", []field{
			{"action", open(SimpleTypeText)},
			{"instanceID", SimpleTypeGUID},
			{"parameters", SimpleTypeText},
			{"softwareAgent", SimpleTypeAgentName},
			{"when", SimpleTypeDate},
		}),
		structType("stRef", "http://ns.adobe.com/xap/1.0/sType/ResourceRef#", "ResourceRef", []field{
			{"instanceID", SimpleTypeGUID},
			{"documentID", SimpleTypeGUID},
			{"versionID", SimpleTypeText},
			{"renditionClass", SimpleTypeRenditionClass},
			{"renditionParams", SimpleTypeText},
			{"manager", SimpleTypeAgentName},
			{"managerVariant", SimpleTypeText},
			{"manageTo", SimpleTypeURI},
			{"manageUI", SimpleTypeURI},
		}),
		structType("stVer", "http://ns.adobe.com/xap/1.0/sType/Version#", "Version", []field{
			{"comments", SimpleTypeText},
			{"event", PredefinedStructResourceEvent},
			{"modifyDate", SimpleTypeDate},
			{"modifier", SimpleTypeProperName},
			{"version", SimpleTypeText},
		}),
		structType("stJob", "http://ns.adobe.com/xap/1.0/sType/Job#", "Job", []field{
			{"name", SimpleTypeText},
			{"id", SimpleTypeText},
			{"url", SimpleTypeURL},
		}),
	}
}

// predefinedStructType is predefined_struct_type in krilla
// src/interchange/metadata/predefined.rs. Do not change the type that it
// returns.
//
// The fields of the predefined structure types, as listed in TechNote
// 0008, chapter 3. Returns nil for types whose fields aren't known.
func predefinedStructType(ty PredefinedStruct) *StructType {
	if ty > PredefinedStructJob {
		// Colorant and Font.
		return nil
	}
	return &predefinedStructTypes[ty]
}
