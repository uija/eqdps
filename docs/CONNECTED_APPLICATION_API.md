# EQLDB Connected Application API

The EQL Log Parser is a public client. It uses the client ID
`eql-log-parser`; this value is an identifier, not a secret.

All responses use JSON over HTTPS. Device connection requests use JSON and
inventory uploads use `multipart/form-data`. Connection codes expire after ten
minutes. The parser must wait at least the returned `interval` between token
requests.

## 1. Start a connection

```http
POST /api/v1/device/connect/
Content-Type: application/json

{
  "client_id": "eql-log-parser",
  "device_name": "Desktop PC"
}
```

Successful response:

```json
{
  "device_code": "private-high-entropy-code",
  "user_code": "ABCD-EFGH",
  "verification_uri": "https://eqldb.org/connect/",
  "verification_uri_complete": "https://eqldb.org/connect/?code=ABCD-EFGH",
  "expires_in": 600,
  "interval": 5
}
```

The parser should display `user_code` and open `verification_uri_complete` in
the user's browser. The browser page handles EQLDB login and asks the user to
allow or cancel the connection.

## 2. Wait for approval

```http
POST /api/v1/device/token/
Content-Type: application/json

{
  "client_id": "eql-log-parser",
  "device_code": "private-high-entropy-code"
}
```

While the browser request is waiting for a decision, the API returns HTTP 400:

```json
{
  "error": "authorization_pending",
  "error_description": "The user has not handled the connection request yet."
}
```

The other terminal errors are `access_denied` and `expired_token`. HTTP 429
returns `slow_down` and a `Retry-After` header. A parser must stop polling after
any terminal error.

After approval, exactly one token request succeeds:

```json
{
  "access_token": "eqldb_private-application-token",
  "token_type": "Bearer",
  "scope": "inventory:upload",
  "connection_id": "opaque-connection-id"
}
```

The access token is shown only in this response. The parser must store it as a
secret. EQLDB stores only its SHA-256 hash. Repeating the token request after a
successful exchange returns `expired_token`.

## 3. Upload inventory

Send the inventory export as `inventory_file`. Classes use the three-letter
codes documented in `WHO_METADATA.md` and can be sent as repeated `classes[]`
fields or as one comma-separated `classes` field. Race uses the displayed
`/who` race name. Level and race are optional.

```http
POST /api/v1/inventory/upload/
Authorization: Bearer eqldb_private-application-token
Content-Type: multipart/form-data; boundary=...

inventory_file=@Character_Server-Inventory.txt
classes[]=PAL
classes[]=MNK
classes[]=BRD
race=Dwarf
level=50
```

With two or three distinct valid classes, EQLDB immediately assigns the
snapshot to that class combination. A newly created loadout inherits the
account permission defaults. Level and race are stored when supplied but are
not required. The race is stored only when it matches EQLDB’s playable-race
catalogue. A missing or unknown race leaves the loadout without a race,
replacing any race previously stored for that loadout. Unknown values are
aggregated for administrator review because `/who` can report illusions.
Successful response:

```json
{
  "status": "completed",
  "character": "Wyrmberg",
  "server": "rivervale",
  "profile_url": "/profiles/account/rivervale/wyrmberg/loadout/"
}
```

With zero or one class, the file is accepted as a pending upload and can be
finished on the account page:

```json
{
  "status": "pending",
  "character": "Wyrmberg",
  "server": "rivervale",
  "message": "The inventory was uploaded and needs a loadout assignment."
}
```

The file is always required. Invalid class codes, duplicate classes, malformed
race values, and invalid levels reject the request instead of being silently
ignored. A printable but unknown race is accepted as described above. The
normal upload switch, file limits, parser validation, and upload rate limits
also apply to API uploads.

## Using and revoking the token

The token identifies its EQLDB account and grants only `inventory:upload`.
Users can view and revoke individual parser installations in the Connected
apps section of their account page. Revocation takes effect immediately.
