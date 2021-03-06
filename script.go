package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type Player struct {
	Skill             float32
	Rank              int
	Streak            int
	Pieces            int
	GamesLeft         int
	GamesPlayed       int
	FailedMatchMaking int
	RankProgression   []RP
}

type RP struct {
	Rank        int
	GamesPlayed int
}

func NewPlayer(skill float32, games int) Player {
	player := Player{}
	player.Skill = skill
	player.GamesLeft = games
	player.Rank = 30
	player.RankProgression = make([]RP, 1)
	player.RankProgression[0] = RP{Rank: 30, GamesPlayed: 0}

	return player
}

func initPlayers(count int, gamesPlayed int) []Player {
	players := make([]Player, count)

	for i := 0; i < count; i++ {
		players[i] = NewPlayer(rand.Float32(), int(rand.Float32()*float32(gamesPlayed)))
	}

	return players
}

func main() {
	log.SetOutput(os.Stderr)
	rand.Seed(time.Now().UnixNano())

	numPlayers := 1000
	numGames := 300

	log.Println("Playing ", numPlayers, " players.")

	players := initPlayers(numPlayers, numGames)
	playersWithGames := make([]int, 100)

	for i := 0; i < numPlayers; i++ {
		playersWithGames = append(playersWithGames, i)
	}

	for len(playersWithGames) > 1 {
		playerA := int(rand.Float32() * float32(len(playersWithGames)))
		a := playersWithGames[playerA]

		matchedPlayers := make([]int, 0)
		for i := 0; i < len(playersWithGames); i++ {
			//Matchmaking
			//if players[playersWithGames[i]].Rank <= players[a].Rank+1 && players[playersWithGames[i]].Rank >= players[a].Rank-1 && playersWithGames[i] != a {
			//	matchedPlayers = append(matchedPlayers, playersWithGames[i])
			//}
			//No Matchmaking
			if playersWithGames[i] != a {
				matchedPlayers = append(matchedPlayers, playersWithGames[i])
			}
		}
		if len(matchedPlayers) > 0 {
			playerB := int(rand.Float32() * float32(len(matchedPlayers)))
			b := playersWithGames[playerB]

			playMatch(&players[a], &players[b])
		} else {
			players[a].FailedMatchMaking++
			if players[a].FailedMatchMaking > 10 {
				log.Println("Player failed matchmaking, rank ", players[a].Rank)
				players[a].GamesLeft = 0
			}
		}

		playersWithGames = make([]int, 0)
		for i, player := range players {
			if player.GamesLeft > 0 {
				playersWithGames = append(playersWithGames, i)
			}
		}
	}

	file, err := os.Create("results.csv")
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, player := range players {
		err := writer.Write([]string{strconv.Itoa(player.GamesPlayed), fmt.Sprintf("%f", player.Skill), strconv.Itoa(player.Rank)})
		checkError("Cannot write to file", err)
	}
}

func playMatch(a *Player, b *Player) int {
	match := rand.Float32() * (a.Skill + b.Skill)

	matchOutcome := 0

	if match < a.Skill {
		matchOutcome = -1
	} else if match > a.Skill {
		matchOutcome = 1
	}

	if matchOutcome < 1 {
		addWin(a)
	} else {
		addLoss(a)
	}

	if matchOutcome > -1 {
		addWin(b)
	} else {
		addLoss(b)
	}

	return matchOutcome
}

func addWin(player *Player) bool {
	//Modify GamesPlayed
	player.GamesLeft--
	player.GamesPlayed++
	player.FailedMatchMaking = 0
	//Modify Streak
	if player.Streak < 0 {
		player.Streak = 1
	} else {
		player.Streak++
	}
	//Modify Pieces / Rank
	if player.Streak >= 3 && player.Rank > 7 {
		player.Pieces += 2
	} else {
		player.Pieces += 1
	}
	if player.Pieces >= 5 {
		if player.Rank != 0 {
			player.Rank--
			player.Pieces -= 5

			player.RankProgression = append(player.RankProgression, RP{Rank: player.Rank, GamesPlayed: player.GamesPlayed})
		}
	}

	if player.GamesLeft == 0 {
		return false
	}

	return true
}

func addLoss(player *Player) bool {
	//Modify GamesPlayed
	player.GamesLeft--
	player.GamesPlayed++
	player.FailedMatchMaking = 0
	//Modify Streak
	if player.Streak > 0 {
		player.Streak = -1
	} else {
		player.Streak--
	}
	//Modify Pieces / Rank
	if player.Rank > 25 {
		player.Streak = 0
	} else if (player.Rank > 14 && player.Streak < -1) || (player.Rank <= 14) {
		player.Streak = 0
		if player.Pieces > 0 {
			player.Pieces--
		} else {
			player.Pieces += 5
			player.Rank++
		}
	}

	if player.GamesLeft == 0 {
		return false
	}

	return true
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
