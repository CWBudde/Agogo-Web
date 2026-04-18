package model

import "encoding/json"

// BlendIfChannel models a Photoshop "Blend If" channel slider.
// Values are [lowHard, lowSoft, highSoft, highHard] in the 0..255 domain.
// A hard cutoff has lowHard == lowSoft and highSoft == highHard.
type BlendIfChannel [4]float64

type BlendIfRange struct {
	Gray  BlendIfChannel `json:"gray"`
	Red   BlendIfChannel `json:"red"`
	Green BlendIfChannel `json:"green"`
	Blue  BlendIfChannel `json:"blue"`
}

type BlendChannelsMask struct {
	R bool `json:"r"`
	G bool `json:"g"`
	B bool `json:"b"`
}

type BlendIfConfig struct {
	ThisLayer       BlendIfRange      `json:"thisLayer"`
	UnderlyingLayer BlendIfRange      `json:"underlyingLayer"`
	Channels        BlendChannelsMask `json:"channels"`
}

func DefaultBlendIfConfig() *BlendIfConfig {
	return &BlendIfConfig{
		ThisLayer: BlendIfRange{
			Gray:  defaultBlendIfChannel(),
			Red:   defaultBlendIfChannel(),
			Green: defaultBlendIfChannel(),
			Blue:  defaultBlendIfChannel(),
		},
		UnderlyingLayer: BlendIfRange{
			Gray:  defaultBlendIfChannel(),
			Red:   defaultBlendIfChannel(),
			Green: defaultBlendIfChannel(),
			Blue:  defaultBlendIfChannel(),
		},
		Channels: BlendChannelsMask{R: true, G: true, B: true},
	}
}

func CloneBlendIfConfig(config *BlendIfConfig) *BlendIfConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	return &cloned
}

func NormalizeBlendIfConfig(config *BlendIfConfig) *BlendIfConfig {
	if config == nil {
		return DefaultBlendIfConfig()
	}
	normalized := *config
	normalized.ThisLayer = normalizeBlendIfRange(normalized.ThisLayer)
	normalized.UnderlyingLayer = normalizeBlendIfRange(normalized.UnderlyingLayer)
	return &normalized
}

// UnmarshalJSON accepts both the current BlendIfConfig shape and the legacy
// shape from project archives ({"gray":[min,max], ...}) so older documents
// continue to load.
func (c *BlendIfConfig) UnmarshalJSON(data []byte) error {
	type newShape struct {
		ThisLayer       BlendIfRange      `json:"thisLayer"`
		UnderlyingLayer BlendIfRange      `json:"underlyingLayer"`
		Channels        BlendChannelsMask `json:"channels"`
	}
	type legacyShape struct {
		Gray  [2]float64 `json:"gray"`
		Red   [2]float64 `json:"red"`
		Green [2]float64 `json:"green"`
		Blue  [2]float64 `json:"blue"`
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	if _, ok := probe["thisLayer"]; ok {
		var parsed newShape
		if err := json.Unmarshal(data, &parsed); err != nil {
			return err
		}
		c.ThisLayer = parsed.ThisLayer
		c.UnderlyingLayer = parsed.UnderlyingLayer
		c.Channels = parsed.Channels
		if _, present := probe["channels"]; !present {
			c.Channels = BlendChannelsMask{R: true, G: true, B: true}
		}
		return nil
	}

	var legacy legacyShape
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	c.ThisLayer = BlendIfRange{
		Gray:  BlendIfChannel{legacy.Gray[0], legacy.Gray[0], legacy.Gray[1], legacy.Gray[1]},
		Red:   BlendIfChannel{legacy.Red[0], legacy.Red[0], legacy.Red[1], legacy.Red[1]},
		Green: BlendIfChannel{legacy.Green[0], legacy.Green[0], legacy.Green[1], legacy.Green[1]},
		Blue:  BlendIfChannel{legacy.Blue[0], legacy.Blue[0], legacy.Blue[1], legacy.Blue[1]},
	}
	c.UnderlyingLayer = BlendIfRange{
		Gray:  defaultBlendIfChannel(),
		Red:   defaultBlendIfChannel(),
		Green: defaultBlendIfChannel(),
		Blue:  defaultBlendIfChannel(),
	}
	c.Channels = BlendChannelsMask{R: true, G: true, B: true}
	return nil
}

func defaultBlendIfChannel() BlendIfChannel {
	return BlendIfChannel{0, 0, 255, 255}
}

func normalizeBlendIfRange(r BlendIfRange) BlendIfRange {
	return BlendIfRange{
		Gray:  normalizeBlendIfChannel(r.Gray),
		Red:   normalizeBlendIfChannel(r.Red),
		Green: normalizeBlendIfChannel(r.Green),
		Blue:  normalizeBlendIfChannel(r.Blue),
	}
}

func normalizeBlendIfChannel(channel BlendIfChannel) BlendIfChannel {
	values := [4]float64{
		clampBlendIfValue(channel[0]),
		clampBlendIfValue(channel[1]),
		clampBlendIfValue(channel[2]),
		clampBlendIfValue(channel[3]),
	}
	if values[1] < values[0] {
		values[1] = values[0]
	}
	if values[2] < values[1] {
		values[2] = values[1]
	}
	if values[3] < values[2] {
		values[3] = values[2]
	}
	return BlendIfChannel(values)
}

func clampBlendIfValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}
