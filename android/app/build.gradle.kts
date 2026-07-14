import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    // Плагина kotlin-android здесь нет намеренно: начиная с AGP 9.0 поддержка Kotlin
    // встроена в сам AGP, и попытка применить его валит сборку.
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.ksp)
    alias(libs.plugins.hilt)
    alias(libs.plugins.ktlint)
}

// Адрес бэкенда и общий секрет не хардкодятся (CLAUDE.md, «Отклонения», п. 3):
// значения берутся из local.properties (не коммитится) или из переменных окружения —
// последнее нужно CI, где local.properties нет.
val localProperties =
    Properties().apply {
        val file = rootProject.file("local.properties")
        if (file.exists()) {
            file.inputStream().use { load(it) }
        }
    }

fun buildSetting(
    key: String,
    fallback: String,
): String = localProperties.getProperty(key) ?: System.getenv(key) ?: fallback

android {
    namespace = "com.stacktraceo.leaguecompanion"

    // 37, а не 36: androidx 2026 года (core-ktx 1.19, lifecycle 2.11, hilt-navigation 1.4)
    // отказывается собираться против более старого SDK. targetSdk при этом остаётся 36 —
    // это независимые вещи: compileSdk даёт доступ к API, targetSdk включает новое
    // поведение рантайма.
    compileSdk = 37

    defaultConfig {
        applicationId = "com.stacktraceo.leaguecompanion"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // 10.0.2.2 — alias хост-машины внутри эмулятора Android. localhost там
        // означает сам эмулятор, поэтому подставить его нельзя.
        buildConfigField("String", "BASE_URL", "\"${buildSetting("LC_BASE_URL", "http://10.0.2.2:8080/")}\"")
        buildConfigField("String", "API_KEY", "\"${buildSetting("LC_API_KEY", "")}\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

ksp {
    // Схема Room в git — чтобы изменения структуры БД были видны в диффе.
    arg("room.schemaLocation", "$projectDir/schemas")
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)
    implementation(libs.androidx.navigation.compose)

    implementation(libs.hilt.android)
    ksp(libs.hilt.compiler)
    implementation(libs.androidx.hilt.navigation.compose)

    implementation(libs.room.runtime)
    implementation(libs.room.ktx)
    ksp(libs.room.compiler)

    implementation(libs.retrofit)
    implementation(libs.retrofit.serialization)
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging)
    implementation(libs.kotlinx.serialization.json)

    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    testImplementation(libs.mockwebserver)
}
