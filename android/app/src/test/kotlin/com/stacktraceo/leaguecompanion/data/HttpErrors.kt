package com.stacktraceo.leaguecompanion.data

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.ResponseBody.Companion.toResponseBody
import retrofit2.HttpException
import retrofit2.Response

/**
 * Ошибка бэкенда в том виде, в каком её бросает Retrofit.
 *
 * Собирать её приходится вручную: у [HttpException] нет конструктора «код + тело»,
 * а тесты разных слоёв (маппер ошибок, репозитории, ViewModel'и) должны получать
 * ровно один и тот же объект — иначе легко проверить обработку ошибки, которой
 * в бою не бывает.
 */
fun httpException(
    code: Int,
    body: String,
    headers: Map<String, String> = emptyMap(),
): HttpException {
    val rawBuilder =
        okhttp3.Response
            .Builder()
            .request(Request.Builder().url("http://10.0.2.2:8080/api/v1/summoners/x").build())
            .protocol(Protocol.HTTP_1_1)
            .code(code)
            .message("error")

    headers.forEach { (name, value) -> rawBuilder.header(name, value) }

    return HttpException(
        Response.error<Any>(body.toResponseBody("application/json".toMediaType()), rawBuilder.build()),
    )
}
