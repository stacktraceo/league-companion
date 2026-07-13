package com.stacktraceo.leaguecompanion.data

/**
 * Компактный ответ Match-V5. Форма сверена с живым ответом бэкенда (86 КБ, 155 полей
 * на участника) — сюда вынесено только то, что читает клиент, плюс пара лишних полей
 * специально: они проверяют, что `ignoreUnknownKeys` действительно включён.
 *
 * Целиком реальный ответ в репозиторий не тащим: это 86 КБ с чужими puuid ради
 * структуры, которая описывается тридцатью строками.
 */
fun matchDetailJson(
    matchId: String = "KR_1",
    gameDuration: Long = 2523,
    gameEndTimestamp: Long? = 1_785_429_021_994,
    positions: List<String> = listOf("TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"),
    trackedPuuid: String = TEST_PUUID,
): String {
    val end = gameEndTimestamp?.let { """"gameEndTimestamp": $it,""" }.orEmpty()

    val participants =
        listOf(100, 200).flatMap { teamId ->
            positions.mapIndexed { index, position ->
                val puuid = if (teamId == 100 && index == 0) trackedPuuid else "puuid-$teamId-$index"
                """
                {
                  "puuid": "$puuid",
                  "riotIdGameName": "Player$teamId$index",
                  "riotIdTagline": "TAG",
                  "championName": "Champion$index",
                  "champLevel": 18,
                  "teamId": $teamId,
                  "teamPosition": "$position",
                  "win": ${teamId == 100},
                  "kills": ${index + 1},
                  "deaths": 2,
                  "assists": 7,
                  "totalMinionsKilled": 200,
                  "neutralMinionsKilled": 15,
                  "goldEarned": 12000,
                  "totalDamageDealtToChampions": 25000,
                  "visionScore": 30,
                  "item0": 3142,
                  "perks": {"statPerks": {"defense": 5011}}
                }
                """.trimIndent()
            }
        }

    return """
        {
          "metadata": {"matchId": "$matchId", "dataVersion": "2"},
          "info": {
            "gameCreation": 1785426498772,
            "gameDuration": $gameDuration,
            $end
            "gameMode": "CLASSIC",
            "queueId": 420,
            "gameVersion": "26.13.1",
            "teams": [
              {
                "teamId": 100,
                "win": true,
                "bans": [{"pickTurn": 1, "championId": 41}],
                "objectives": {
                  "champion": {"first": true, "kills": 45},
                  "tower": {"first": false, "kills": 6},
                  "dragon": {"first": true, "kills": 4},
                  "baron": {"first": true, "kills": 2},
                  "inhibitor": {"first": false, "kills": 1},
                  "riftHerald": {"first": false, "kills": 0},
                  "atakhan": {"first": false, "kills": 0}
                }
              },
              {
                "teamId": 200,
                "win": false,
                "bans": [],
                "objectives": {
                  "champion": {"first": false, "kills": 30},
                  "tower": {"first": true, "kills": 3},
                  "dragon": {"first": false, "kills": 1},
                  "baron": {"first": false, "kills": 0},
                  "inhibitor": {"first": false, "kills": 0},
                  "riftHerald": {"first": true, "kills": 1}
                }
              }
            ],
            "participants": [${participants.joinToString(",")}]
          }
        }
        """.trimIndent()
}
