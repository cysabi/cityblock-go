package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/twpayne/go-geos"
)

type PingRequest struct {
	Lobby  string       `json:"lobby"`
	Player string       `json:"player"`
	Points [][2]float64 `json:"points"` // [lng, lat] pairs, ascending order (oldest first)
}

// PingEndpoint receives location pings from a client and updates game state.
// POST /ping { lobby, player, points }
func PingEndpoint(w http.ResponseWriter, r *http.Request) {
	// validate request
	w.Header().Set("Content-Type", "application/json")
	var req PingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Lobby == "" || req.Player == "" {
		http.Error(w, "lobby and player are required", http.StatusBadRequest)
		return
	}
	if len(req.Points) == 0 {
		http.Error(w, "points is required", http.StatusBadRequest)
		return
	}

	lobbiesMu.Lock()

	// get game & player
	game := lobbies[req.Lobby]
	if game == nil {
		lobbiesMu.Unlock()
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}
	var player *Player
	for _, p := range game.Players {
		if p.Tag == req.Player {
			player = p
			break
		}
	}
	if player == nil {
		lobbiesMu.Unlock()
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	// update player state
	affected, err := updatePlayerState(game, player, req.Points)
	if err != nil {
		lobbiesMu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// emit to ws
	messages := PreparePlayerUpdates(req.Lobby, affected)
	lobbiesMu.Unlock()

	go BroadcastPrepared(req.Lobby, messages)

	json.NewEncoder(w).Encode(nil)
}

func updatePlayerState(game *Game, p *Player, points [][2]float64) ([]string, error) {
	// prepend LatestPoint so OSRM can connect to previous position
	if p.LatestPoint != nil {
		points = append([][2]float64{*p.LatestPoint}, points...)
	} else {
		p.LatestPoint = &points[len(points)-1]
		return []string{p.Tag}, nil
	}

	// // snap to roads
	// segments, err := snapToRoads(points)
	// if err != nil {
	// 	return nil, err
	// }

	// // add each matching as a line to the trail
	// for _, seg := range segments {
	// 	if len(seg) < 2 {
	// 		continue
	// 	}
	// 	line := geos.NewLineString(toGeosCoords(seg))
	// 	if p.Trail == nil {
	// 		p.Trail = geos.NewCollection(geos.TypeIDMultiLineString, []*geos.Geom{line})
	// 	} else {
	// 		p.Trail = p.Trail.Union(line).UnaryUnion()
	// 	}
	// }

	// straight direct lines
	if len(points) >= 2 {
		line := geos.NewLineString(toGeosCoords(points))
		if p.Trail == nil {
			p.Trail = geos.NewCollection(geos.TypeIDMultiLineString, []*geos.Geom{line})
		} else {
			p.Trail = p.Trail.Union(line).UnaryUnion()
		}
	}

	// update LatestPoint from last matching
	last := points[len(points)-1]
	p.LatestPoint = &last

	// detect holes in trail → claim enclosed areas
	affected := []string{p.Tag}
	affected = append(affected, claimHoles(game, p)...)

	return affected, nil
}

func claimHoles(game *Game, player *Player) []string {
	if player.Trail == nil {
		return nil
	}

	// get claimed areas
	claimed, cuts, dangles, _ := player.Trail.PolygonizeFull()
	if claimed.IsEmpty() {
		return nil
	}

	// add to player's claimed territory
	if player.Claimed == nil {
		player.Claimed = claimed
	} else {
		player.Claimed = player.Claimed.Union(claimed)
	}

	// trail becomes only the leftover lines (cuts + dangles)
	player.Trail = geos.NewCollection(geos.TypeIDGeometryCollection, []*geos.Geom{cuts, dangles}).UnaryUnion()

	// subtract from opponents
	var affected []string
	for _, opponent := range game.Players {
		if opponent.Tag == player.Tag {
			continue
		}
		if opponent.Trail != nil {
			opponent.Trail = viralSubtract(opponent.Trail, claimed)
		}
		if opponent.Claimed != nil {
			opponent.Claimed = opponent.Claimed.Difference(claimed)
		}
		affected = append(affected, opponent.Tag)
	}
	return affected
}

func viralSubtract(trail, claimed *geos.Geom) *geos.Geom {
	trail = trail.Difference(claimed)
	infected := claimed
	for {
		var keep []*geos.Geom
		spread := false
		for i := range trail.NumGeometries() {
			line := trail.Geometry(i)
			if line.Intersects(infected) {
				infected = infected.Union(line)
				spread = true
			} else {
				keep = append(keep, line)
			}
		}
		if !spread || len(keep) == 0 {
			return trail
		}
		trail = geos.NewCollection(geos.TypeIDMultiLineString, keep)
	}
}

func snapToRoads(points [][2]float64) ([][][2]float64, error) {
	coords := ""
	for i, p := range points {
		if i > 0 {
			coords += ";"
		}
		coords += fmt.Sprintf("%f,%f", p[0], p[1])
	}

	uri := "https://router.project-osrm.org/match/v1/foot/" + coords + "?geometries=geojson&tidy=true"

	fmt.Printf("uri: %#v\n", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Matchings []struct {
			Geometry struct {
				Coordinates [][2]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"matchings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Matchings) == 0 {
		return [][][2]float64{points}, nil
	}
	segments := make([][][2]float64, len(result.Matchings))
	for i, m := range result.Matchings {
		segments[i] = m.Geometry.Coordinates
	}
	return segments, nil
}
