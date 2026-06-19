package state

// Connection lifecycle reducers. The runtime registers/unregisters
// connections via EvConnOpened / EvConnClosed; subscribe / unsubscribe
// flip the broadcast filter on a connected client.

func reduceConnOpened(s State, e EvConnOpened) (State, []Effect) {
	if e.ConnID > s.NextConnID {
		s.NextConnID = e.ConnID
	}
	return s, nil
}

func reduceConnClosed(s State, e EvConnClosed) (State, []Effect) {
	_, hasSub := s.Subscribers[e.ConnID]
	inner, hasSurface := s.SurfaceSubs[e.ConnID]
	if !hasSub && !hasSurface {
		return s, nil
	}
	var effs []Effect
	if hasSub {
		s.Subscribers = cloneSubscribers(s.Subscribers)
		delete(s.Subscribers, e.ConnID)
	}
	if hasSurface {
		// Emit one Stop per (ConnID, SessionID) before dropping the
		// outer map entry so the runtime can tear down each relay.
		// Iteration order is non-deterministic — tests must compare as
		// sets, not slices.
		s.SurfaceSubs = cloneSurfaceSubs(s.SurfaceSubs)
		for sid := range inner {
			effs = append(effs, EffSurfaceSubscribeStop{ConnID: e.ConnID, SessionID: sid})
		}
		delete(s.SurfaceSubs, e.ConnID)
	}
	return s, effs
}

func reduceSubscribe(s State, e EvCmdSubscribe) (State, []Effect) {
	s.Subscribers = cloneSubscribers(s.Subscribers)
	s.Subscribers[e.ConnID] = Subscriber{
		ConnID:  e.ConnID,
		Filters: append([]string(nil), e.Filters...),
	}
	return s, []Effect{
		okResp(e.ConnID, e.ReqID, nil),
		// Send the initial sessions snapshot so the client doesn't
		// need to follow up with list-sessions.
		EffBroadcastSessionsChanged{},
	}
}

func reduceUnsubscribe(s State, e EvCmdUnsubscribe) (State, []Effect) {
	if _, ok := s.Subscribers[e.ConnID]; ok {
		s.Subscribers = cloneSubscribers(s.Subscribers)
		delete(s.Subscribers, e.ConnID)
	}
	return s, []Effect{okResp(e.ConnID, e.ReqID, nil)}
}
