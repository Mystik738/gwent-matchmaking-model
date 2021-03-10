package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"
)

const (
	//Substantial Model changes
	Derank         = false //Allows players to de-rank on losses. Currently disabled in the game, but was part of older ranking systems
	Learn          = true  //Allows players to learn as they play more games.
	GamesPerSeason = 360   //Max (+ SeasonalVariance), average will be half this. Number from https://forums.cdprojektred.com/index.php?threads/deep-analysis-of-journey-performed-by-game-director-himself.11028497/

	//Minor Model changes. Note that these are not linear variables, so the descriptions aren't quite accurate.
	SkillOffsetScale = 100 //How many games we expect the average player to learn the game. Set at 100 due to MMR requiring 100 games (25 per 4 factions) to mature, but anyone's guess.
	LearnScale       = 2.0 //Allows some players to learn faster than others
	LearnFactor      = 1.0 //Affects all players. Larger increases learning speed, but also increases the "just don't get it" factor for struggling players
	SeasonalVariance = 100 //The maximum number of games play may change between seasons for players. Players randomly receive their own variance bounded by this.

	//Procedural changes
	Debug             = false
	Seasons           = 6
	PlayersPerSeason  = 5000
	FailedMatchMaking = 10 //Matchmaking attempts before a player ragequits the season, mostly to prevent small user pools from infinite loops
)

type Player struct {
	Id                int
	Rank              int
	Streak            int
	Pieces            int
	GamesLeft         int
	GamesPlayed       int
	GamesPerSeason    int
	SeasonalVariance  int
	FailedMatchMaking int
	RankProgression   []RankProgression
	Skill             Skill
}

type RankProgression struct {
	Rank        int
	GamesPlayed int
}

type Skill struct {
	max    float64
	offset int
	rate   float64
	Calc   func(skill *Skill, gamesPlayed int) float64
}

func CalcSkill(skill *Skill, gamesPlayed int) float64 {
	if Learn {
		return skill.max * float64(.5+math.Atan(float64(gamesPlayed+skill.offset)/float64(skill.rate))/math.Pi)
	}
	return skill.max
}

func NewPlayer(id int, skill float64, games int, variance int) Player {
	player := Player{}
	player.Id = id
	player.GamesPerSeason = games
	player.SeasonalVariance = variance
	player.Rank = 30
	player.RankProgression = make([]RankProgression, 1)
	for i := 30; i >= player.Rank; i-- {
		player.RankProgression[0] = RankProgression{Rank: i, GamesPlayed: 0}
	}

	player.Skill = Skill{
		max:    rand.Float64(),
		offset: int((rand.Float64() - .5) * float64(SkillOffsetScale)),
		rate:   float64(SkillOffsetScale / (1.0 + (rand.Float64() * (LearnScale - 1.0)))), //This looks complicated, but pins the learning rate to the skill offset rate
		Calc:   CalcSkill}

	setPlayerForSeason(&player, false)

	return player
}

func initPlayers(count int, gamesPlayed int, startId int) []Player {
	players := make([]Player, count)

	for i := 0; i < count; i++ {
		players[i] = NewPlayer(i+startId, rand.Float64(), int(rand.Float64()*float64(gamesPlayed)), int(rand.Float64()*float64(SeasonalVariance)))
	}

	return players
}

func setPlayerForSeason(p *Player, resetRank bool) {
	if resetRank {
		if p.Rank < 28 {
			p.Rank = p.Rank + 3
		} else {
			p.Rank = 30
		}
	}
	p.GamesLeft = p.GamesPerSeason + int((rand.Float64()-0.5)*float64(p.SeasonalVariance))
	if p.GamesLeft < 0 {
		p.GamesLeft = 0
	}
}

func main() {
	log.SetOutput(os.Stderr)
	rand.Seed(time.Now().UnixNano())

	log.Println("Playing", Seasons, "season(s), adding", PlayersPerSeason, "each season with an average", GamesPerSeason/2, "games played per season.")

	players := make([]Player, 0)

	for s := 0; s < Seasons; s++ {
		//Season init
		players = append(players, initPlayers(PlayersPerSeason, GamesPerSeason, s*PlayersPerSeason)...)
		playersWithGames := make([]int, 0)
		playersWGBR := make([][]int, 31)

		//Get skill of top 500 Pro Rank
		proPlayers := make([]*Player, 0)
		for i := 0; i < len(players); i++ {
			if players[i].Rank == 0 {
				proPlayers = append(proPlayers, &players[i])
			}
		}

		//Find the cut for Pro Rank. This isn't fMMR, but gets the top skilled.
		proCutOff := 0.0
		if len(proPlayers) > 500 {
			sort.Slice(proPlayers, func(i, j int) bool {
				return proPlayers[i].Skill.Calc(&proPlayers[i].Skill, proPlayers[i].GamesPlayed) > proPlayers[j].Skill.Calc(&proPlayers[j].Skill, proPlayers[j].GamesPlayed)
			})

			proCutOff = proPlayers[499].Skill.Calc(&proPlayers[499].Skill, proPlayers[499].GamesPlayed)
		}

		if Debug {
			log.Println("ProRank skill cutoff:", proCutOff)
		}

		playersSittingOut := 0
		for i := 0; i < len(players); i++ {
			if s != 0 {
				//If players are in the Pro Rank Top 500, don't derank. Hell, don't even play them for efficiency, just grant then their games
				if players[i].Rank == 0 && proCutOff < players[i].Skill.Calc(&players[i].Skill, players[i].GamesPlayed) {
					setPlayerForSeason(&players[i], false)
					players[i].GamesPlayed += players[i].GamesLeft
					players[i].GamesLeft = 0
				} else {
					setPlayerForSeason(&players[i], true)
				}
			}
			if players[i].GamesLeft > 0 {
				playersWithGames = append(playersWithGames, i)
				playersWGBR[players[i].Rank] = append(playersWGBR[players[i].Rank], i)
			} else {
				playersSittingOut++
			}
		}

		if Debug {
			log.Println(playersSittingOut, "players are sitting out this season.")
		}

		//Start playing games
		for len(playersWithGames) > 1 {
			aGamesIndex := int(rand.Float64() * float64(len(playersWithGames)))
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

			//If there aren't any players in a's rank, add ranks above and below
			if len(playersWGBR[aRank])-1 == 0 {
				if aRank != 30 {
					numMatched += len(playersWGBR[aRank+1])
					playersIdBelow += len(playersWGBR[aRank+1])
				}
				if aRank != 0 {
					numMatched += len(playersWGBR[aRank-1])
				}
			}
			//If we matched, find a player and play
			if numMatched > 0 {
				bRankedIndex := int(rand.Float64() * float64(numMatched))
				bRank := aRank
				if len(playersWGBR[aRank])-1 == 0 {
					bRank = aRank + 1
					if aRank == 30 {
						bRank--
					} else if bRankedIndex >= len(playersWGBR[aRank+1]) {
						bRank--
						bRankedIndex -= len(playersWGBR[aRank+1])
					}
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

				//Move players in their ranks if they ranked or remove them if they're out of games
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

					if aRank-1 != 0 {
						playersWGBR[aRank-1] = append(playersWGBR[aRank-1], aId)
					} else {
						//ProRank players don't need to progress in this model, just grant them their games
						players[aId].GamesPlayed += players[aId].GamesLeft
						players[aId].GamesLeft = 0

						playersWithGames[aGamesIndex] = playersWithGames[len(playersWithGames)-1]
						playersWithGames = playersWithGames[:len(playersWithGames)-1]
					}
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

						if bRank-1 != 0 {
							playersWGBR[bRank-1] = append(playersWGBR[bRank-1], bId)
						} else {
							//ProRank players don't need to progress in this model, just grant them their games
							players[bId].GamesPlayed += players[bId].GamesLeft
							players[bId].GamesLeft = 0

							playersWithGames[bGamesIndex] = playersWithGames[len(playersWithGames)-1]
							playersWithGames = playersWithGames[:len(playersWithGames)-1]
						}
					} else if bRanked == -1 {
						playersWGBR[bRank][bRankedIndex] = playersWGBR[bRank][len(playersWGBR[bRank])-1]
						playersWGBR[bRank] = playersWGBR[bRank][:len(playersWGBR[bRank])-1]

						playersWGBR[bRank+1] = append(playersWGBR[bRank+1], bId)
					}
				}
			} else { //We didn't find a match, ding a, and with enough dings, ragequit
				players[aId].FailedMatchMaking++
				if players[aId].FailedMatchMaking > FailedMatchMaking {
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
				//Ensure that our rank arrays have players with the right ranks. This is very slow
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

		endStats(&players, s)
	}
}

func endStats(p *[]Player, season int) {
	playersBR := make([][]int, 31)
	for i := 0; i < len(*p); i++ {
		playersBR[(*p)[i].Rank] = append(playersBR[(*p)[i].Rank], i)
	}

	fileName := ""
	if Derank {
		fileName += "Derank"
	} else {
		fileName += "NoDerank"
	}
	if Learn {
		fileName += "Learn"
	} else {
		fileName += "NoLearn"
	}

	//file, err := os.Create(fileName + strconv.Itoa(season) + ".csv")
	file, err := os.Create(fileName + ".csv")
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{"Rank", "Player Count", "Average Games Played", "Average Skill", "Std Dev", "Average Progression Count"})
	log.Println("Season", season, "Rankings:")
	checkError("Cannot write to file", err)

	for r := 0; r < len(playersBR); r++ {
		gp := 0
		skill := (float64)(0.0)
		gpAll := 0
		cnt := len(playersBR[r])
		cntAll := 0

		for i := 0; i < cnt; i++ {
			gp += (*p)[playersBR[r][i]].GamesPlayed
			pSkill := &(*p)[playersBR[r][i]].Skill
			skill += (*p)[playersBR[r][i]].Skill.Calc(pSkill, (*p)[playersBR[r][i]].GamesPlayed)
		}

		avg := skill / (float64)(cnt)

		stddev := 0.0
		for i := 0; i < cnt; i++ {
			pSkill := &(*p)[playersBR[r][i]].Skill
			stddev += math.Pow(float64((*p)[playersBR[r][i]].Skill.Calc(pSkill, (*p)[playersBR[r][i]].GamesPlayed)-avg), 2)
		}
		stddev = math.Sqrt(stddev / float64(cnt))

		for rp := r - 1; rp >= 0; rp-- {
			cntAll += len(playersBR[rp])
			for i := 0; i < len(playersBR[rp]); i++ {
				gpAll += (*p)[playersBR[rp][i]].RankProgression[31-r].GamesPlayed - 1
			}
		}

		if cnt > 0 {
			log.Println("Rank", r, "Players:", cnt, "GamesPlayed:", gp/cnt, "Skill:", avg, "GamesToProgress:", (gp+gpAll)/(cnt+cntAll))

			err := writer.Write([]string{strconv.Itoa(r), strconv.Itoa(cnt), fmt.Sprintf("%f", float64(gp)/float64(cnt)), fmt.Sprintf("%f", avg), fmt.Sprintf("%f", stddev), fmt.Sprintf("%f", float64(gp+gpAll)/float64(cnt+cntAll))})
			checkError("Cannot write to file", err)
		} else {
			log.Println("Rank", r, "Players: 0 GamesPlayed: 0 Skill: n/a GamesToProgress: n/a")
		}
	}
}

func playMatch(a *Player, b *Player) (int, int) {
	aSkill := &a.Skill
	bSkill := &b.Skill
	match := rand.Float64() * (a.Skill.Calc(aSkill, a.GamesPlayed) + b.Skill.Calc(bSkill, b.GamesPlayed))
	aRankedUp := 0
	bRankedUp := 0

	matchOutcome := 0

	if match < a.Skill.Calc(aSkill, a.GamesPlayed) {
		matchOutcome = -1
	} else if match > a.Skill.Calc(aSkill, a.GamesPlayed) {
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
	//This is a little strange. You need more than 5 pieces to rank up, but when you do you rank with 1 piece already.
	if player.Pieces > 5 {
		if player.Rank != 0 {
			player.Rank--
			player.Pieces -= 5
			rankedUp = 1
			if player.RankProgression[len(player.RankProgression)-1].Rank > player.Rank {
				player.RankProgression = append(player.RankProgression, RankProgression{Rank: player.Rank, GamesPlayed: player.GamesPlayed})
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
			//Can't derank due to loss in ProRank, just lose MMR
			if Derank && player.Rank != 0 {
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
