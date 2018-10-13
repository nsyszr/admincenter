-----LUA-----
cert = [[-----BEGIN CERTIFICATE-----
MIICLzCCAbWgAwIBAgIJAIvA4E2mRohWMAoGCCqGSM49BAMCMFQxCzAJBgNVBAYT
AkRFMQswCQYDVQQIDAJCWTEUMBIGA1UEBwwLU2V1YmVyc2RvcmYxETAPBgNVBAoM
CFRTQ0hPS0tPMQ8wDQYDVQQDDAZzZXJ2ZXIwHhcNMTgxMDAxMTkwMTU5WhcNMjgw
OTI4MTkwMTU5WjBUMQswCQYDVQQGEwJERTELMAkGA1UECAwCQlkxFDASBgNVBAcM
C1NldWJlcnNkb3JmMREwDwYDVQQKDAhUU0NIT0tLTzEPMA0GA1UEAwwGc2VydmVy
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEosGK7khnIMY+Y8MRl+a6x9azQ4zHjRAJ
wsrnm3j3Q+zW/p0NTL1F73h7KgFb+RsX+Zx4xQeFiFZfg0n4d2FvFiB4KHRMvX8x
kOZ/uMuCQoHoaOF9mwfrCCrt/aCqUDXMo1MwUTAdBgNVHQ4EFgQUooxqUvcsi+qp
TceiG5ORNh5EkBQwHwYDVR0jBBgwFoAUooxqUvcsi+qpTceiG5ORNh5EkBQwDwYD
VR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNoADBlAjEA2rle9w5qTP8l0q7KCRu8
JIeR2O9gAf30n+NAxvVOvN3eKIbCAHmDA3OLf9WvB4iPAjAhC/+/MLzvQbp/MyIY
HmEdv9zkzUkU/FlQiH0GjsAMfklYv9XkAP78FWzyHlG5YJ4=
-----END CERTIFICATE-----]]

hash = cli("status.sysdetail.system.hash")
print("Hash before: " .. hash)

out = cli("administration.certificates.public_certs.cert.add")
print("1: " .. out)

out = cli("administration.certificates.public_certs.cert[last].name=cert_acws")
print("2: " .. out)

out = cli("administration.certificates.public_certs.cer[last].certificate=" .. cert)
print("3: " .. out)

out = cli("administration.profiles.activate")
print("4: " .. out)

sleep(2)
hash = cli("status.sysdetail.system.hash")
print("Hash after: " .. hash)
-----LUA-----
