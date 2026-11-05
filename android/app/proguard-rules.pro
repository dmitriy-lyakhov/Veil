# gomobile / JNI: классы и методы, которые вызываются из нативного кода
-keep class go.** { *; }
-keep class veilcore.** { *; }
-keepclasseswithmembernames class * { native <methods>; }
# zxing-android-embedded
-keep class com.google.zxing.** { *; }
-keep class com.journeyapps.barcodescanner.** { *; }
-dontwarn com.google.zxing.**
