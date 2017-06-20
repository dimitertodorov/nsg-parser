$packageFiles = @("README.md","LICENSE.txt","nsg-parser.yml.sample","VERSION","nsg-parser.exe")
if (-not (Test-Path env:GITHUB_TOKEN)) {
    Write-Host '$env:GITHUB_TOKEN must be set'
    exit 2
}
if (-not (Test-Path env:GOPATH)) {
    Write-Host '$env:GOPATH must be set'
    exit 3
}
#Get Prerequisites
go get github.com/prometheus/promu
go get github.com/Masterminds/glide
go get github.com/aktau/github-release

go get -d github.com/dimitertodorov/nsg-parser
Set-Location "$env:GOPATH/src/github.com/dimitertodorov/nsg-parser"
promu build
$version = (.\nsg-parser.exe version -s).Trim()
if($version -Match '[0-9]+.[0-9]+.[0-9]+.*'){
    Write-Host "Built Version $version. Packaging and pushing to Github"
    $archiveName = "nsg-parser-$version.windows-amd64.zip"
    Compress-Archive -Path $packageFiles -DestinationPath $archiveName -Update
    github-release upload --user dimitertodorov --repo nsg-parser --tag "v$version" --name $archiveName --file $archiveName
}