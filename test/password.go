package main

import (
    "encoding/base64"
    "fmt"
    "log"

    jose "github.com/square/go-jose/v3"
)

func main() {
    // Исходный пароль
    plaintext := []byte("password")

    // Симметричный ключ 32 байта (256 бит)
    // Лучше сгенерировать случайный ключ и защитить его
    keyBase64 := "RgzRPJRasPwSmKi5HXkvRTr9ZyjGadO4w5jSrbVnmEk="
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        log.Fatal("Failed to decode base64 key:", err)
    }

    // Создаем объект для шифрования JOSE с алгоритмом "dir" (direct) и "A256GCM"
    enc, err := jose.NewEncrypter(
        jose.A256GCM,
        jose.Recipient{
            Algorithm: jose.DIRECT,
            Key:       key,
        },
        nil,
    )
    if err != nil {
        log.Fatal("Failed to create encrypter:", err)
    }

    // Шифруем пароль
    jweObject, err := enc.Encrypt(plaintext)
    if err != nil {
        log.Fatal("Failed to encrypt:", err)
    }

    // Получаем сериализованную строку JWE в формате compact
    serialized := jweObject.FullSerialize()

    fmt.Println("Encrypted password (JOSE JWE compact):")
    fmt.Println(serialized)
}