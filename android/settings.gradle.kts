pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    // Репозитории объявлены только здесь: модуль, добавивший свой, сломает сборку,
    // а не утащит зависимость из непонятного места.
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "league-companion"

include(":app")
