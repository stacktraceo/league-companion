# Retrofit + kotlinx.serialization: интерфейс API и @Serializable-классы читаются
# через generated-сериализаторы, поэтому R8 не должен их переименовывать.
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response

# Сигнатуры дженериков нужны Retrofit'у для разбора возвращаемых типов.
-keepattributes Signature, InnerClasses, EnclosingMethod
-keepattributes RuntimeVisibleAnnotations, RuntimeVisibleParameterAnnotations
