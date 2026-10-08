plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "work.deeka.lists"
    compileSdk = 35

    defaultConfig {
        applicationId = "work.deeka.lists"
        minSdk = 30
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        debug {
            // The Vite dev server on this machine, reached through
            // `adb reverse tcp:5175 tcp:5175`. Using "localhost" keeps the page a
            // secure context, so the Secure refresh cookie works over plain HTTP.
            buildConfigField("String", "BASE_URL", "\"http://localhost:5175\"")
            manifestPlaceholders["cleartext"] = "true"
        }
        release {
            buildConfigField("String", "BASE_URL", "\"${providers.gradleProperty("LISTS_URL").get()}\"")
            manifestPlaceholders["cleartext"] = "false"
            // ponytail: signed with the debug key so the APK installs directly on a
            // phone. Create a real keystore before publishing to the Play Store.
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    buildFeatures {
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}
