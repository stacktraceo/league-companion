package com.stacktraceo.leaguecompanion

import android.app.Application
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import dagger.hilt.android.HiltAndroidApp
import javax.inject.Inject

@HiltAndroidApp
class LeagueCompanionApp :
    Application(),
    SingletonImageLoader.Factory {
    // Coil спрашивает загрузчик лениво, при первой картинке, - то есть уже после
    // onCreate(), где Hilt заполняет поле.
    @Inject
    lateinit var imageLoader: ImageLoader

    override fun newImageLoader(context: PlatformContext): ImageLoader = imageLoader
}
