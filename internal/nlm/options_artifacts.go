package nlm

// artifactOptions is the options block of the artifact-customization call
// (sqTeoe): [2,null,null,[1,null×9,[1]],[[1,4,8,10,14,2,3,6]]]. It is NOT the
// HDR block from requestOptions(): live capture 2026-10-13 shows a shorter
// client block ([1] instead of [1,3]) followed by the list of artifact type
// codes the UI asks presets for.
func artifactOptions() []any {
	return []any{
		2, nil, nil,
		[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
		[]any{[]any{1, 4, 8, 10, 14, 2, 3, 6}},
	}
}

// studioChoicesParams is the full sqTeoe request: [options, null, 1]
// (live capture 2026-10-13).
func studioChoicesParams() []any { return []any{artifactOptions(), nil, 1} }
