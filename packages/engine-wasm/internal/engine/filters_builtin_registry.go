package engine

func init() {
	registerBuiltinFilters()
}

func registerBuiltinFilters() {
	RegisterFilter(FilterDef{
		ID:       "invert",
		Name:     "Invert",
		Category: FilterCategoryOther,
	}, filterInvert)

	RegisterFilter(FilterDef{
		ID:        "gaussian-blur",
		Name:      "Gaussian Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterGaussianBlur)

	RegisterFilter(FilterDef{
		ID:        "brightness-contrast",
		Name:      "Brightness/Contrast",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterBrightnessContrast)

	RegisterFilter(FilterDef{
		ID:        "unsharp-mask",
		Name:      "Unsharp Mask",
		Category:  FilterCategorySharpen,
		HasDialog: true,
	}, filterUnsharpMask)

	RegisterFilter(FilterDef{
		ID:        "add-noise",
		Name:      "Add Noise",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterAddNoise)

	RegisterFilter(FilterDef{
		ID:        "high-pass",
		Name:      "High Pass",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterHighPass)

	RegisterFilter(FilterDef{
		ID:        "emboss",
		Name:      "Emboss",
		Category:  FilterCategoryStylize,
		HasDialog: true,
	}, filterEmboss)

	RegisterFilter(FilterDef{
		ID:       "solarize",
		Name:     "Solarize",
		Category: FilterCategoryStylize,
	}, filterSolarize)

	RegisterFilter(FilterDef{
		ID:       "find-edges",
		Name:     "Find Edges",
		Category: FilterCategoryStylize,
	}, filterFindEdges)

	RegisterFilter(FilterDef{
		ID:        "box-blur",
		Name:      "Box Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterBoxBlur)

	RegisterFilter(FilterDef{
		ID:       "sharpen",
		Name:     "Sharpen",
		Category: FilterCategorySharpen,
	}, filterSharpen)

	RegisterFilter(FilterDef{
		ID:       "sharpen-more",
		Name:     "Sharpen More",
		Category: FilterCategorySharpen,
	}, filterSharpenMore)

	RegisterFilter(FilterDef{
		ID:        "median",
		Name:      "Median",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterMedian)

	RegisterFilter(FilterDef{
		ID:       "despeckle",
		Name:     "Despeckle",
		Category: FilterCategoryNoise,
	}, filterDespeckle)

	RegisterFilter(FilterDef{
		ID:        "minimum",
		Name:      "Minimum",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterMinimum)

	RegisterFilter(FilterDef{
		ID:        "maximum",
		Name:      "Maximum",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterMaximum)

	RegisterFilter(FilterDef{
		ID:        "ripple",
		Name:      "Ripple",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterRipple)

	RegisterFilter(FilterDef{
		ID:        "twirl",
		Name:      "Twirl",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterTwirl)

	RegisterFilter(FilterDef{
		ID:        "offset",
		Name:      "Offset",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterOffset)

	RegisterFilter(FilterDef{
		ID:        "polar-coordinates",
		Name:      "Polar Coordinates",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterPolarCoordinates)

	RegisterFilter(FilterDef{
		ID:        "motion-blur",
		Name:      "Motion Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterMotionBlur)

	RegisterFilter(FilterDef{
		ID:        "radial-blur",
		Name:      "Radial Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterRadialBlur)

	RegisterFilter(FilterDef{
		ID:        "surface-blur",
		Name:      "Surface Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterSurfaceBlur)

	RegisterFilter(FilterDef{
		ID:        "smart-sharpen",
		Name:      "Smart Sharpen",
		Category:  FilterCategorySharpen,
		HasDialog: true,
	}, filterSmartSharpen)

	RegisterFilter(FilterDef{
		ID:        "reduce-noise",
		Name:      "Reduce Noise",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterReduceNoise)

	RegisterFilter(FilterDef{
		ID:        "lens-correction",
		Name:      "Lens Correction",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterLensCorrection)
}
