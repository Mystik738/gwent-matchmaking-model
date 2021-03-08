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

const (
	Derank    = false
	Debug     = false
	PlayerNum = 5000
	GameNum   = 3000
)

type Player struct {
	Id                int
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

func NewPlayer(id int, skill float32, games int) Player {
	player := Player{}
	player.Id = id
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
		players[i] = NewPlayer(i, rand.Float32(), int(rand.Float32()*float32(gamesPlayed)))
	}

	return players
}

func main() {
	log.SetOutput(os.Stderr)
	rand.Seed(time.Now().UnixNano())

	log.Println("Playing", PlayerNum, "players, average", GameNum/2, "games played.")

	players := initPlayers(PlayerNum, GameNum)
	playersWithGames := make([]int, 0)
	//Players with games by rank
	playersWGBR := make([][]int, 31)

	for i := 0; i < PlayerNum; i++ {
		playersWithGames = append(playersWithGames, i)
		playersWGBR[30] = append(playersWGBR[30], i)
	}

	for len(playersWithGames) > 1 {
		aGamesIndex := int(rand.Float32() * float32(len(playersWithGames)))
		aId := playersWithGames[aGamesIndex]
		aRank := players[aId].Rank

		//Matchmaking
		aRankedIndex := -1
		numMatched := len(playersWGBR[aRank]) - 1
		playersIdBelow := 0
		for i := 0; i < len(playersWGBR[aRank]); i++ {
			if players[playersWGBR[aRank][i]].Id != aId {
				playersIdBelow++
			} else {
				aRankedIndex = i
				break
			}
		}

		if aRank != 30 {
			numMatched += len(playersWGBR[aRank+1])
			playersIdBelow += len(playersWGBR[aRank+1])
		}
		if aRank != 0 {
			numMatched += len(playersWGBR[aRank-1])
		}
		if numMatched > 0 {
			bRankedIndex := int(rand.Float32() * float32(numMatched))
			bRank := aRank + 1
			if aRank == 30 {
				bRank--
			} else if bRankedIndex >= len(playersWGBR[aRank+1]) {
				bRank--
				bRankedIndex -= len(playersWGBR[aRank+1])
			}
			if bRank == aRank && bRankedIndex >= aRankedIndex {
				bRankedIndex++

				if bRankedIndex > len(playersWGBR[aRank])-1 {
					bRank--
					bRankedIndex -= len(playersWGBR[aRank])
				}
			}

			bId := playersWGBR[bRank][bRankedIndex]

			aRanked, bRanked := playMatch(&players[aId], &players[bId])

			if players[aId].GamesLeft <= 0 {
				if Debug {
					log.Println("Removing", aId, "from lists")
				}
				//Remove from lists
				playersWithGames[aGamesIndex] = playersWithGames[len(playersWithGames)-1]
				playersWithGames = playersWithGames[:len(playersWithGames)-1]

				playersWGBR[aRank][aRankedIndex] = playersWGBR[aRank][len(playersWGBR[aRank])-1]
				playersWGBR[aRank] = playersWGBR[aRank][:len(playersWGBR[aRank])-1]
			} else if aRanked == 1 {
				playersWGBR[aRank][aRankedIndex] = playersWGBR[aRank][len(playersWGBR[aRank])-1]
				playersWGBR[aRank] = playersWGBR[aRank][:len(playersWGBR[aRank])-1]

				playersWGBR[aRank-1] = append(playersWGBR[aRank-1], aId)
			} else if aRanked == -1 {
				playersWGBR[aRank][aRankedIndex] = playersWGBR[aRank][len(playersWGBR[aRank])-1]
				playersWGBR[aRank] = playersWGBR[aRank][:len(playersWGBR[aRank])-1]

				playersWGBR[aRank+1] = append(playersWGBR[aRank+1], aId)
			}
			if players[bId].GamesLeft <= 0 || bRanked != 0 {
				//If player A moved, we need to refind b's rankedIndex
				if (players[aId].GamesLeft <= 0 || aRanked != 0) && bRank == aRank {
					for i := 0; i < len(playersWGBR[bRank]); i++ {
						if players[playersWGBR[bRank][i]].Id == bId {
							bRankedIndex = i
							break
						}
					}
				}
				bGamesIndex := -1
				for i := 0; i < len(playersWithGames); i++ {
					if playersWithGames[i] == bId {
						bGamesIndex = i
						break
					}
				}

				if players[bId].GamesLeft <= 0 {
					if Debug {
						log.Println("Removing", bId, "from lists")
					}
					playersWithGames[bGamesIndex] = playersWithGames[len(playersWithGames)-1]
					playersWithGames = playersWithGames[:len(playersWithGames)-1]

					playersWGBR[bRank][bRankedIndex] = playersWGBR[bRank][len(playersWGBR[bRank])-1]
					playersWGBR[bRank] = playersWGBR[bRank][:len(playersWGBR[bRank])-1]
				} else if bRanked == 1 {
					playersWGBR[bRank][bRankedIndex] = playersWGBR[bRank][len(playersWGBR[bRank])-1]
					playersWGBR[bRank] = playersWGBR[bRank][:len(playersWGBR[bRank])-1]

					playersWGBR[bRank-1] = append(playersWGBR[bRank-1], bId)
				} else if bRanked == -1 {
					playersWGBR[bRank][bRankedIndex] = playersWGBR[bRank][len(playersWGBR[bRank])-1]
					playersWGBR[bRank] = playersWGBR[bRank][:len(playersWGBR[bRank])-1]

					playersWGBR[bRank+1] = append(playersWGBR[bRank+1], bId)
				}
			}
		} else {
			players[aId].FailedMatchMaking++
			if players[aId].FailedMatchMaking > 10 {
				if Debug {
					log.Println("Player", aId, "failed matchmaking, rank ", players[aId].Rank)
				}
				players[aId].GamesLeft = 0

				playersWithGames[aGamesIndex] = playersWithGames[len(playersWithGames)-1]
				playersWithGames = playersWithGames[:len(playersWithGames)-1]

				playersWGBR[aRank][aRankedIndex] = playersWGBR[aRank][len(playersWGBR[aRank])-1]
				playersWGBR[aRank] = playersWGBR[aRank][:len(playersWGBR[aRank])-1]
			}
		}

		if Debug {
			for r := 0; r < len(playersWGBR); r++ {
				for i := 0; i < len(playersWGBR[r]); i++ {
					if players[playersWGBR[r][i]].Rank != r {
						log.Println(playersWGBR[r][i], players[playersWGBR[r][i]].Rank, r)
						panic("rank mismatch")
					}
				}
			}
		}
	}

	endStats(&players)
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

func endStats(p *[]Player) {
	playersBR := make([][]int, 31)
	for i := 0; i < len(*p); i++ {
		playersBR[(*p)[i].Rank] = append(playersBR[(*p)[i].Rank], i)
	}

	for r := 0; r < len(playersBR); r++ {
		gp := 0
		skill := (float32)(0.0)
		gpAll := 0
		cnt := len(playersBR[r])
		cntAll := 0

		for i := 0; i < cnt; i++ {
			gp += (*p)[playersBR[r][i]].GamesPlayed
			skill += (*p)[playersBR[r][i]].Skill
		}

		for rp := r - 1; rp >= 0; rp-- {
			cntAll += len(playersBR[rp])
			for i := 0; i < len(playersBR[rp]); i++ {
				//if Debug {
				//log.Println((*p)[playersBR[rp][i]].RankProgression)
				//}
				gpAll += (*p)[playersBR[rp][i]].RankProgression[31-r].GamesPlayed - 1
			}
		}

		log.Println("Rank", r, ":", cnt, "|", gp/cnt, skill/(float32)(cnt), "|", (gp+gpAll)/(cnt+cntAll))
	}
}

func playMatch(a *Player, b *Player) (int, int) {
	match := rand.Float32() * (a.Skill + b.Skill)
	aRankedUp := 0
	bRankedUp := 0

	matchOutcome := 0

	if match < a.Skill {
		matchOutcome = -1
	} else if match > a.Skill {
		matchOutcome = 1
	}

	if matchOutcome < 1 {
		_, aRankedUp = addWin(a)
	} else {
		_, aRankedUp = addLoss(a)
	}

	if matchOutcome > -1 {
		_, bRankedUp = addWin(b)
	} else {
		_, bRankedUp = addLoss(b)
	}

	return aRankedUp, bRankedUp
}

func addWin(player *Player) (bool, int) {
	rankedUp := 0
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
	if player.Pieces > 5 {
		if player.Rank != 0 {
			player.Rank--
			player.Pieces -= 5
			rankedUp = 1
			if player.RankProgression[len(player.RankProgression)-1].Rank > player.Rank {
				player.RankProgression = append(player.RankProgression, RP{Rank: player.Rank, GamesPlayed: player.GamesPlayed})
			}
		}
	}

	if player.GamesLeft == 0 {
		return false, rankedUp
	}

	return true, rankedUp
}

func addLoss(player *Player) (bool, int) {
	rankedDown := 0
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
			if Derank {
				player.Pieces += 5
				player.Rank++
				rankedDown = -1
			}
		}
	}

	if player.GamesLeft == 0 {
		return false, rankedDown
	}

	return true, rankedDown
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
