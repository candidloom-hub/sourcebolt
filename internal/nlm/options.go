package nlm

// requestOptions is the client-options block the web app sends with most
// RPCs: [2,null,[1],[1,null×9,[1,3]]] (upstreams call it HDR). Live capture
// on 2026-10-13 showed it on wXbhsf, rLM1Ne, VfAZjd, CCqFvf, izAoDd, hPTbtc,
// khqZz, cFji9, I3xc3c, JFMDGd, e3bVqc, EylDcb and the chat stream; the
// short [2] form some upstreams send is outdated.
func requestOptions() []any {
	return []any{2, nil, []any{1}, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1, 3}}}
}
