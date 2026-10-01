# Builds the disk-space Plugin: the linux/amd64 backend binary and the ZIP that
# "Import a Plugin…" accepts.
#
# The binary sits beside manifest.json and widget.js because that is the path
# the Manifest runs (`./backend`). The Go source is src/, because a file and a
# directory cannot share a name in one folder.
#
# Run from anywhere:  pwsh apps/api/plugins/disk-space/build.ps1

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

# The one target this Plugin promises (D13). CGO_ENABLED=0 is the difference
# between a static binary that runs on alpine and one that fails on musl.
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

Write-Host "building backend (linux/amd64)…"
go build -C "$here/src" -trimpath -ldflags "-s -w -X main.version=0.1.0" -o "$here/backend" .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

# The ZIP is built with System.IO.Compression and NOT with Compress-Archive.
# Compress-Archive writes no Unix mode into a zip entry, so the panel would
# unpack a backend it cannot execute — and core would then log
# "backend exited (permission denied); restarting in 2s" forever while the
# Plugins section looked healthy. The Unix mode lives in the high 16 bits of
# the entry's external attributes; 0o755 for the binary, 0o644 for the rest.
Add-Type -AssemblyName System.IO.Compression.FileSystem
$dist = Join-Path $here "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$zip = Join-Path $dist "disk-space.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }

# Anything that is not the Plugin: the previous ZIP, the journal written at
# runtime, and the ignore file itself.
$excluded = @("dist", "state", ".archiveignore")

$archive = [System.IO.Compression.ZipFile]::Open($zip, "Create")
try {
  Get-ChildItem -Path $here -Recurse -File | Where-Object {
    $relative = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    -not ($excluded | Where-Object { $relative -eq $_ -or $relative.StartsWith("$_/") })
  } | ForEach-Object {
    $relative = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    $entry = $archive.CreateEntry("disk-space/$relative", [System.IO.Compression.CompressionLevel]::Optimal)
    # 0x1ED is 0o755 (the backend) and 0x1A4 is 0o644 (everything else):
    # PowerShell has no octal literals, so the modes are written in hex.
    $entry.ExternalAttributes = $(if ($relative -eq "backend") { 0x1ED } else { 0x1A4 }) -shl 16
    $entry.LastWriteTime = $_.LastWriteTime
    $input = [System.IO.File]::OpenRead($_.FullName)
    try {
      $output = $entry.Open()
      try { $input.CopyTo($output) } finally { $output.Dispose() }
    } finally { $input.Dispose() }
  }
} finally { $archive.Dispose() }

# maxPluginArchiveBytes is 32 MiB in apps/api/server.go; failing at 24 MiB is
# failing before the owner has picked the file, not after.
$size = (Get-Item $zip).Length
if ($size -gt 25165824) {
  throw "disk-space.zip is $size bytes; the panel refuses anything over 32 MiB"
}
Write-Host ("wrote {0} ({1:N1} MiB)" -f $zip, ($size / 1MB))
