param(
    [string]$Version = "v0.12.0",
    [string]$Destination = "internal/fpdfpatch"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$modCache = (& go env GOMODCACHE).Trim()
if (-not $modCache) {
    throw "go env GOMODCACHE returned an empty path"
}

$source = Join-Path $modCache "codeberg.org/go-pdf/fpdf@$Version"
$target = Join-Path $repoRoot $Destination
$patch = Join-Path $repoRoot "patches/fpdf-xobject-metadata.patch"

if (-not (Test-Path -LiteralPath $source)) {
    throw "fpdf module cache path not found: $source"
}
if (-not (Test-Path -LiteralPath $patch)) {
    throw "patch file not found: $patch"
}

if (Test-Path -LiteralPath $target) {
    Remove-Item -LiteralPath $target -Recurse -Force
}

New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force | Out-Null
Copy-Item -LiteralPath $source -Destination $target -Recurse
Get-ChildItem -LiteralPath $target -Recurse -Force | ForEach-Object {
    if (-not $_.PSIsContainer) {
        $_.Attributes = [System.IO.FileAttributes]::Normal
    }
}

function Replace-Once {
    param(
        [string]$Path,
        [string]$Old,
        [string]$New
    )
    $text = [System.IO.File]::ReadAllText($Path)
    if (-not $text.Contains($Old)) {
        throw "patch context not found in $Path"
    }
    $text = $text.Replace($Old, $New)
    [System.IO.File]::WriteAllText($Path, $text)
}

$defPath = Join-Path $target "def.go"
$fpdfPath = Join-Path $target "fpdf.go"

Replace-Once $defPath @'
	scale float64 // Document scale factor
	dpi   float64 // Dots-per-inch found from image file (png only)
	i     string  // SHA-1 checksum of the above values.
'@ @'
	scale float64 // Document scale factor
	dpi   float64 // Dots-per-inch found from image file (png only)
	i     string  // SHA-1 checksum of the above values.

	extraDict map[string]string // Additional image XObject dictionary string entries.
'@

Replace-Once $fpdfPath @'
func (f *Fpdf) GetImageInfo(imageStr string) (info *ImageInfoType) {
	return f.images[imageStr]
}

'@ @'
func (f *Fpdf) GetImageInfo(imageStr string) (info *ImageInfoType) {
	return f.images[imageStr]
}

// SetImageDictionaryString stores a string entry in a registered image XObject
// dictionary. The key must not include a leading slash.
func (f *Fpdf) SetImageDictionaryString(imageStr, key, value string) {
	info := f.images[imageStr]
	if info == nil {
		f.err = fmt.Errorf("image has not been registered: %s", imageStr)
		return
	}
	if info.extraDict == nil {
		info.extraDict = map[string]string{}
	}
	info.extraDict[key] = value
}

func keySortStringMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func pdfLiteralString(s string) string {
	var b strings.Builder
	b.WriteByte('(')
	for _, r := range s {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(')')
	return b.String()
}

'@

Replace-Once $fpdfPath @'
	if info.smask != nil {
		f.outf("/SMask %d 0 R", f.n+1)
	}
	f.outf("/Length %d>>", len(info.data))
'@ @'
	if info.smask != nil {
		f.outf("/SMask %d 0 R", f.n+1)
	}
	for _, key := range keySortStringMap(info.extraDict) {
		f.outf("/%s %s", key, pdfLiteralString(info.extraDict[key]))
	}
	f.outf("/Length %d>>", len(info.data))
'@

Write-Host "Patched fpdf copied to $Destination"
