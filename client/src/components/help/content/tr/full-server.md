# Tam sunucu (her şeyi kendin)

Tüm mqvi platformunu kendi sunucunda çalıştır — hesaplar, mesajlar, dosyalar ve ses — mqvi.net'ten tamamen bağımsız.

## Gereksinimler

- Linux sunucusu (Ubuntu 22.04+ / Debian 12+), x86_64 ya da arm64
- En az 2 vCPU, 4 GB RAM
- Alan adı **opsiyonel** — alan adı olmadan kurulum betiği ücretsiz bir `sslip.io` hostname kullanır, böylece HTTPS çalışır (tarayıcılar ses/kamerayı düz HTTP üzerinden engeller)

## Tek komutla kurulum

Sunucuna SSH ile bağlan ve şunu çalıştır:

```bash
curl -fsSL https://raw.githubusercontent.com/akinalpfdn/Mqvi/main/deploy/install.sh | sudo bash
```

mqvi'yi nasıl yayınlamak istediğin sorulacak:

1. **Kendi alan adın** (önerilir) — Caddy + Let's Encrypt'i otomatik kurar
2. **sslip.io** — alan adı gerekmez; HTTPS yine kutudan çıktığı gibi çalışır
3. **Sadece HTTP** — yalnızca test için; tarayıcılar mikrofon, kamera ve ekran paylaşımını engeller

Etkileşimsiz bir çalıştırma mı tercih edersin? Flag geç:

```bash
sudo bash install.sh --domain demo.example.com --port 9092 -y   # your own domain
sudo bash install.sh -y                                          # sslip.io, all defaults
sudo bash install.sh --no-tls --port 8080 -y                     # HTTP only
```

## Neyi kurar

Özel bir `mqvi` kullanıcısı ve `/opt/mqvi` dizini, önceden derlenmiş sunucu binary'si (~40 MB — her şey gömülü, Go/Node/Docker gerekmez), LiveKit ses sunucusu, rastgele secret'lar, sıkılaştırılmış systemd servisleri, güvenlik duvarı ve (TLS modlarında) Caddy. **Yeniden çalıştırmak güvenlidir** — mevcut secret'lar korunur.

Kurulum bittiğinde URL'ini yazdırır (`https://yourdomain` ya da `https://1-2-3-4.sslip.io`). **Kayıt olan ilk kullanıcı sahip olur.**

## Çalıştırma

```bash
journalctl -u mqvi-server -f          # follow logs
systemctl restart mqvi-server         # restart

# update to a newer release:
curl -fsSL https://raw.githubusercontent.com/akinalpfdn/Mqvi/main/deploy/install.sh | sudo bash
systemctl restart mqvi-server
```

Verilerin **`/opt/mqvi/data/`** içinde yaşar (veritabanı + yüklenen dosyalar) — **yedeğini al.** Sunucunu masaüstü uygulamasından kullanmak için **Bağlantılar** üzerinden oraya yönlendir (sonraki sayfa).
