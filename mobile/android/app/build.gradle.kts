plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Подпись release: параметры берутся из ~/.gradle/gradle.properties (вне репозитория) или переменных окружения
//   VEIL_KEYSTORE (путь), VEIL_STORE_PASSWORD, VEIL_KEY_ALIAS, VEIL_KEY_PASSWORD
fun sign(name: String): String? = (project.findProperty(name) as String?) ?: System.getenv(name)

android {
    namespace = "net.veil.android"
    compileSdk = 34

    defaultConfig {
        applicationId = "net.veil.android"
        minSdk = 26
        targetSdk = 34
        versionCode = 6
        versionName = "1.1.0"
        ndk { abiFilters += listOf("arm64-v8a", "x86_64") }
    }

    signingConfigs {
        create("release") {
            val ks = sign("VEIL_KEYSTORE")
            if (ks != null) {
                storeFile = file(ks)
                storePassword = sign("VEIL_STORE_PASSWORD")
                keyAlias = sign("VEIL_KEY_ALIAS") ?: "veil"
                keyPassword = sign("VEIL_KEY_PASSWORD") ?: sign("VEIL_STORE_PASSWORD")
                enableV1Signing = false
                enableV2Signing = true
                enableV3Signing = true
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (sign("VEIL_KEYSTORE") != null) signingConfig = signingConfigs.getByName("release")
        }
        debug {
            // отладочная сборка ставится рядом с рабочей и не трогает её профили
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
    packaging { jniLibs { useLegacyPackaging = false } }
}

dependencies {
    // Go-ядро: scripts/build-android.sh|.ps1 кладёт сюда veilcore.aar
    implementation(files("libs/veilcore.aar"))
    // QR-сканер
    implementation("com.journeyapps:zxing-android-embedded:4.3.0")
    // zxing-android-embedded вызывает ContextCompat (запрос разрешения камеры), но не тянет androidx.core транзитивно
    implementation("androidx.core:core:1.12.0")
}
