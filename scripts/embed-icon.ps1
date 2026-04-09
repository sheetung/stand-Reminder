param(
  [Parameter(Mandatory = $true)]
  [string]$ExePath,

  [Parameter(Mandatory = $true)]
  [string]$IconPath
)

$ErrorActionPreference = 'Stop'

$exeFullPath = (Resolve-Path $ExePath).Path
$iconFullPath = (Resolve-Path $IconPath).Path
$iconBytes = [System.IO.File]::ReadAllBytes($iconFullPath)

if ($iconBytes.Length -lt 22) {
  throw 'Invalid ICO file.'
}

function Read-UInt16LE([byte[]]$buffer, [int]$offset) {
  return [BitConverter]::ToUInt16($buffer, $offset)
}

function Read-UInt32LE([byte[]]$buffer, [int]$offset) {
  return [BitConverter]::ToUInt32($buffer, $offset)
}

function Write-UInt16LE([byte[]]$buffer, [int]$offset, [UInt16]$value) {
  $bytes = [BitConverter]::GetBytes($value)
  [Array]::Copy($bytes, 0, $buffer, $offset, 2)
}

function Write-UInt32LE([byte[]]$buffer, [int]$offset, [UInt32]$value) {
  $bytes = [BitConverter]::GetBytes($value)
  [Array]::Copy($bytes, 0, $buffer, $offset, 4)
}

$reserved = Read-UInt16LE $iconBytes 0
$type = Read-UInt16LE $iconBytes 2
$count = Read-UInt16LE $iconBytes 4

if ($reserved -ne 0 -or $type -ne 1 -or $count -lt 1) {
  throw 'Unsupported ICO structure.'
}

$entries = @()
for ($i = 0; $i -lt $count; $i++) {
  $offset = 6 + ($i * 16)
  if ($offset + 16 -gt $iconBytes.Length) {
    throw 'ICO directory is truncated.'
  }

  $width = $iconBytes[$offset + 0]
  $height = $iconBytes[$offset + 1]
  $colorCount = $iconBytes[$offset + 2]
  $reservedByte = $iconBytes[$offset + 3]
  $planes = Read-UInt16LE $iconBytes ($offset + 4)
  $bitCount = Read-UInt16LE $iconBytes ($offset + 6)
  $bytesInRes = Read-UInt32LE $iconBytes ($offset + 8)
  $imageOffset = Read-UInt32LE $iconBytes ($offset + 12)

  if (($imageOffset + $bytesInRes) -gt $iconBytes.Length) {
    throw 'ICO image payload is truncated.'
  }

  $data = New-Object byte[] $bytesInRes
  [Array]::Copy($iconBytes, [int]$imageOffset, $data, 0, [int]$bytesInRes)

  $entries += [pscustomobject]@{
    Width = $width
    Height = $height
    ColorCount = $colorCount
    Reserved = $reservedByte
    Planes = $planes
    BitCount = $bitCount
    BytesInRes = $bytesInRes
    Data = $data
    ID = [UInt16]($i + 1)
  }
}

$groupBytes = New-Object byte[] (6 + (14 * $count))
Write-UInt16LE $groupBytes 0 0
Write-UInt16LE $groupBytes 2 1
Write-UInt16LE $groupBytes 4 $count

for ($i = 0; $i -lt $entries.Count; $i++) {
  $entry = $entries[$i]
  $offset = 6 + ($i * 14)
  $groupBytes[$offset + 0] = [byte]$entry.Width
  $groupBytes[$offset + 1] = [byte]$entry.Height
  $groupBytes[$offset + 2] = [byte]$entry.ColorCount
  $groupBytes[$offset + 3] = [byte]$entry.Reserved
  Write-UInt16LE $groupBytes ($offset + 4) $entry.Planes
  Write-UInt16LE $groupBytes ($offset + 6) $entry.BitCount
  Write-UInt32LE $groupBytes ($offset + 8) $entry.BytesInRes
  Write-UInt16LE $groupBytes ($offset + 12) $entry.ID
}

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class ResourceWriter {
  [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
  public static extern IntPtr BeginUpdateResource(string pFileName, bool bDeleteExistingResources);

  [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
  public static extern bool UpdateResource(IntPtr hUpdate, IntPtr lpType, IntPtr lpName, ushort wLanguage, byte[] lpData, uint cbData);

  [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
  public static extern bool EndUpdateResource(IntPtr hUpdate, bool fDiscard);
}
"@

$updateHandle = [ResourceWriter]::BeginUpdateResource($exeFullPath, $false)
if ($updateHandle -eq [IntPtr]::Zero) {
  throw "BeginUpdateResource failed: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
}

$discard = $true
try {
  foreach ($entry in $entries) {
    $ok = [ResourceWriter]::UpdateResource(
      $updateHandle,
      [IntPtr]3,
      [IntPtr][int]$entry.ID,
      0,
      $entry.Data,
      [uint32]$entry.Data.Length
    )
    if (-not $ok) {
      throw "UpdateResource RT_ICON failed: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
    }
  }

  $ok = [ResourceWriter]::UpdateResource(
    $updateHandle,
    [IntPtr]14,
    [IntPtr]1,
    0,
    $groupBytes,
    [uint32]$groupBytes.Length
  )
  if (-not $ok) {
    throw "UpdateResource RT_GROUP_ICON failed: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
  }

  $discard = $false
}
finally {
  $endOk = [ResourceWriter]::EndUpdateResource($updateHandle, $discard)
  if (-not $endOk) {
    throw "EndUpdateResource failed: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
  }
}