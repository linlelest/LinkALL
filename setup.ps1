#requires -Version 5.1

$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()
[Console]::InputEncoding = [Text.UTF8Encoding]::new()

$rootDir   = if ($PSScriptRoot) { $PSScriptRoot } else { Get-Location | Select-Object -ExpandProperty Path }
$script:serverDir = Join-Path $rootDir 'server'
$script:envFile    = Join-Path $script:serverDir '.env'
$script:envExample = Join-Path $script:serverDir '.env.example'
$script:lang       = 'zh-CN'

function T($zh,$en) { if ($script:lang -eq 'en-US') {$en} else {$zh} }

function Step($m) { Write-Host ">>> $m" -ForegroundColor Cyan }
function Ok($m) { Write-Host "[OK] $m" -ForegroundColor Green }
function Warn($m) { Write-Host "[WARN] $m" -ForegroundColor Yellow }
function Err($m) { Write-Host "[ERR] $m" -ForegroundColor Red }
function Info($m) { Write-Host $m -ForegroundColor Magenta -NoNewline }

function Pause-Menu {
    Write-Host ''
    Write-Host (T '按 Enter 返回主菜单...' 'Press Enter to return...') -ForegroundColor DarkGray
    $null = Read-Host
}

function Read-Choice($prompt, $choices) {
    Write-Host ''
    Write-Host $prompt -ForegroundColor Green
    for ($i = 0; $i -lt $choices.Count; $i++) {
        Write-Host ('  [{0}] {1}' -f ($i+1), $choices[$i])
    }
    Write-Host '  [0] ' -NoNewline; Write-Host (T '返回' 'Back')
    $input = Read-Host '>>> '
    if ($input -eq '0') { return $null }
    if ($input -match '^\d+$') {
        $n = [int]$input
        if ($n -ge 1 -and $n -le $choices.Count) { return $n - 1 }
    }
    Write-Host (T '输入无效' 'Invalid input') -ForegroundColor Red
    return Read-Choice $prompt $choices
}

function Confirm-YesNo($prompt) {
    $r = Read-Host "$prompt (y/N)"
    return ($r -eq 'y' -or $r -eq 'Y')
}

# ---- .env ----
function Ensure-EnvFile {
    if (-not $script:envFile) {
        if (-not $rootDir) { $rootDir = if ($PSScriptRoot) { $PSScriptRoot } else { Get-Location | Select-Object -ExpandProperty Path } }
        $sd = Join-Path $rootDir 'server'
        $script:envFile    = Join-Path $sd '.env'
        $script:envExample = Join-Path $sd '.env.example'
    }
    if (!(Test-Path $script:envFile)) {
        if (Test-Path $script:envExample) { Copy-Item $script:envExample $script:envFile }
    }
}

function Get-EnvValue($key) {
    Ensure-EnvFile
    if (!(Test-Path $script:envFile)) { return $null }
    foreach ($line in (Get-Content $script:envFile -Encoding UTF8)) {
        if ($line -match "^$key=(.*)") { return $matches[1] }
    }
    return $null
}

function Set-EnvValue($key, $value) {
    Ensure-EnvFile
    if (!(Test-Path $script:envFile)) { return }
    $lines = Get-Content $script:envFile -Encoding UTF8
    $found = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match "^#?$key=") {
            $lines[$i] = "$key=$value"
            $found = $true
        }
    }
    if (!$found) { $lines += "$key=$value" }
    Set-Content $script:envFile $lines -Encoding UTF8
    Ok "$key = $value"
}

function Reset-EnvFile {
    Ensure-EnvFile
    if (Test-Path $script:envFile) { Remove-Item $script:envFile -Force }
    Copy-Item $script:envExample $script:envFile
    Ok (T '.env 已重置为默认值' '.env has been reset to defaults')
}

function Generate-RandomSecret($len) {
    $chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$'
    $chars = $chars + '*-_=+'
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $bytes = [byte[]]::new($len)
    $rng.GetBytes($bytes)
    $result = ''
    for ($i = 0; $i -lt $len; $i++) { $result += $chars[$bytes[$i] % $chars.Length] }
    return $result
}

# ---- dep ----
function Test-Command($name) {
    try { $null = Get-Command $name -ErrorAction Stop; return $true }
    catch { return $false }
}

function Show-DepStatus {
    Write-Host ''
    Write-Host (T '依赖检测:' 'Dependency Check:') -ForegroundColor Magenta
    $deps = @(('Go','go'),('Node.js','node'),('npm','npm'),('pnpm','pnpm'),('Rust/Cargo','cargo'),('JDK','java'),('Gradle','gradle'))
    foreach ($d in $deps) {
        $ok = Test-Command $d[1]
        $icon = if ($ok) { '[OK]' } else { '[--]' }
        if ($ok) {
            $ver = & $d[1] --version 2>&1 | Select-Object -First 1
            Write-Host ('  {0} {1} {2}' -f $icon, $d[0], $ver) -ForegroundColor Green
        } else {
            Write-Host ('  {0} {1} (not found)' -f $icon, $d[0]) -ForegroundColor DarkGray
        }
    }
}

# ========================== MAIN ==========================
function Show-MainMenu {
    Clear-Host
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host '      LinkALL Setup and Build Helper' -ForegroundColor Cyan
    Write-Host '========================================' -ForegroundColor Cyan
    $langMsg = if ($script:lang -eq 'zh-CN') { '当前语言: 中文' } else { 'Current language: English' }
    Write-Host $langMsg
    Write-Host (T '工作目录: ' 'Working dir: ') -NoNewline; Write-Host $rootDir -ForegroundColor White
    Write-Host ''
    $c = @()
    $c += (T '快速设置配置项' 'Configure .env')
    $c += (T '重置所有配置为默认' 'Reset all settings')
    $c += (T '编译部署' 'Build and deploy')
    $c += '切换语言 / Switch language'
    $i = Read-Choice (T '请选择操作:' 'Choose an action:') $c
    if ($null -eq $i) { exit }
    switch ($i) {
        0 { Show-ConfigMenu }
        1 { Reset-EnvFile; Pause-Menu; Show-MainMenu }
        2 { Show-BuildMenu }
        3 { $script:lang = if ($script:lang -eq 'zh-CN') { 'en-US' } else { 'zh-CN' }; Show-MainMenu }
    }
}

# ========================== CONFIG ==========================
function Show-ConfigMenu {
    Clear-Host
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host ('  ' + (T '配置 .env' '.env Configuration')) -ForegroundColor Cyan
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host ''

    # 1 JWT_SECRET
    $cur = Get-EnvValue 'JWT_SECRET'
    Write-Host '[1] JWT_SECRET' -ForegroundColor Yellow
    Write-Host ('    Current: ' + $cur)
    $yn = Confirm-YesNo (T '  生成随机密钥? (y/N)' '  Generate random secret? (y/N)')
    if ($yn) {
        Set-EnvValue 'JWT_SECRET' (Generate-RandomSecret 48)
    }

    # 2 HTTP_ADDR
    $cur = Get-EnvValue 'HTTP_ADDR'
    Write-Host ''
    Write-Host ('[2] HTTP_ADDR (' + $cur + ')')
    $v = Read-Host (T '  监听地址 (留空不变): ' '  Listen address (blank keep): ')
    if ($v) { Set-EnvValue 'HTTP_ADDR' $v }

    # 3 PUBLIC_URL
    $cur = Get-EnvValue 'PUBLIC_URL'
    Write-Host ''
    Write-Host ('[3] PUBLIC_URL (' + $cur + ')')
    $v = Read-Host (T '  公网地址 (留空不变): ' '  Public URL (blank keep): ')
    if ($v) { Set-EnvValue 'PUBLIC_URL' $v }

    # 4 OFFICIAL_SERVER
    $cur = Get-EnvValue 'OFFICIAL_SERVER'
    Write-Host ''
    Write-Host ('[4] OFFICIAL_SERVER (' + $cur + ')')
    $v = Read-Host (T '  官方服务器地址 (留空不变): ' '  Official server (blank keep): ')
    if ($v) { Set-EnvValue 'OFFICIAL_SERVER' $v }

    # 5 JWT_TTL_HOURS
    $cur = Get-EnvValue 'JWT_TTL_HOURS'
    Write-Host ''
    Write-Host ('[5] JWT_TTL_HOURS (' + $cur + ')')
    $v = Read-Host (T '  JWT 有效期小时 (留空不变): ' '  JWT TTL hours (blank keep): ')
    if ($v) { Set-EnvValue 'JWT_TTL_HOURS' $v }

    # 6 TLS
    Write-Host ''
    Write-Host '[6] HTTPS (TLS)'
    $yn = Confirm-YesNo (T '  启用 HTTPS? (y/N)' '  Enable HTTPS? (y/N)')
    if ($yn) {
        $cert = Read-Host (T '  TLS_CERL证书路径): ' '  TLS_CERLcert path): ')
        $key  = Read-Host (T '  TLS_KEY (私钥路径): ' '  TLS_KEY (key path): ')
        if ($cert) { Set-EnvValue 'TLS_CERT' $cert }
        if ($key)  { Set-EnvValue 'TLS_KEY' $key }
    } else {
        Set-EnvValue 'TLS_CERT' ''
        Set-EnvValue 'TLS_KEY' ''
    }

    # 7 TURN
    Write-Host ''
    Write-Host '[7] TURN Server'
    $yn = Confirm-YesNo (T '  配置 TURN? (跳过则只用 STUN) (y/N)' '  Configure TURN? (skip = STUN only) (y/N)')
    if ($yn) {
        $u = Read-Host '  TURN_URL (turn:host:port)'
        if ($u) { Set-EnvValue 'TURN_URL' $u }
        $yn2 = Confirm-YesNo (T '  使用 coturn use-auth-secret? (y/N)' '  Use coturn use-auth-secret? (y/N)')
        if ($yn2) {
            $s = Read-Host '  TURN_SECRET'
            if ($s) { Set-EnvValue 'TURN_SECRET' $s }
        } else {
            $tu = Read-Host '  TURN_USER'; if ($tu) { Set-EnvValue 'TURN_USER' $tu }
            $tc = Read-Host '  TURN_CRED'; if ($tc) { Set-EnvValue 'TURN_CRED' $tc }
        }
    }

    # 8 Argon2
    Write-Host ''
    Write-Host '[8] Argon2id'
    $yn = Confirm-YesNo (T '  调整 Argon2 参数? (y/N)' '  Tweak Argon2 params? (y/N)')
    if ($yn) {
        $v = Read-Host '  ARGON2_TIME (iterations)'; if ($v) { Set-EnvValue 'ARGON2_TIME' $v }
        $v = Read-Host '  ARGON2_MEMORY_KB';           if ($v) { Set-EnvValue 'ARGON2_MEMORY_KB' $v }
        $v = Read-Host '  ARGON2_THREADS';             if ($v) { Set-EnvValue 'ARGON2_THREADS' $v }
    }

    # 9 business defaults
    Write-Host ''
    Write-Host '[9] ' + (T '业务默认开关' 'Business defaults')
    $yn = Confirm-YesNo (T '  修改业务默认开关? (y/N)' '  Change business defaults? (y/N)')
    if ($yn) {
        $curVal = Get-EnvValue 'ALLOW_ANONYMOUS_DEFAULT'
        $yn2 = Confirm-YesNo ('  ' + (T '允许匿名连接? (当前: ' 'Allow anonymous? (current: ') + $curVal + ') (y/N)')
        if ($yn2) { Set-EnvValue 'ALLOW_ANONYMOUS_DEFAULT' 'true' } else { Set-EnvValue 'ALLOW_ANONYMOUS_DEFAULT' 'false' }
        $curVal = Get-EnvValue 'REQUIRE_DEVICE_CODE_DEFAULT'
        $yn2 = Confirm-YesNo ('  ' + (T '需要设备码? (当前: ' 'Require device code? (current: ') + $curVal + ') (y/N)')
        if ($yn2) { Set-EnvValue 'REQUIRE_DEVICE_CODE_DEFAULT' 'true' } else { Set-EnvValue 'REQUIRE_DEVICE_CODE_DEFAULT' 'false' }
    }

    Write-Host ''
    Ok (T '配置完成!' 'Configuration done!')
    Pause-Menu
    Show-MainMenu
}

# ========================== BUILD ==========================
function Show-BuildMenu {
    Clear-Host
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host ('  ' + (T '编译部署' 'Build and Deploy')) -ForegroundColor Cyan
    Write-Host '========================================' -ForegroundColor Cyan
    Show-DepStatus
    Write-Host ''
    $c = @()
    $c += (T '本地编译所有端 (并行)' 'Build all ends (parallel)')
    $c += (T 'GitHub Actions 自动部署' 'GitHub Actions auto-deploy')
    $i = Read-Choice (T '请选择:' 'Choose:') $c
    if ($null -eq $i) { Show-MainMenu; return }
    switch ($i) {
        0 { Invoke-LocalBuild }
        1 { Invoke-GitHubActions }
    }
}

function Invoke-LocalBuild {
    Clear-Host
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host ('  ' + (T '本地编译' 'Local Build')) -ForegroundColor Cyan
    Write-Host '========================================' -ForegroundColor Cyan

    $avail = @()
    if (Test-Command 'go') {
        $avail += @{Name='server';Label=(T '服务端' 'Server');Cmd='go build -trimpath -ldflags "-s -w" -o linkall-server ./cmd/server';Dir=Join-Path $rootDir 'server'}
    }
    if (Test-Command 'npm') {
        $avail += @{Name='web';Label=(T '网页' 'Web');Cmd='npm run build';Dir=Join-Path $rootDir 'web'}
    }
    if (Test-Command 'cargo') {
        $avail += @{Name='tauri';Label=(T 'Windows 被控端' 'Win Host');Cmd='npm run tauri build';Dir=Join-Path $rootDir 'controlled-win'}
    }
    if ((Test-Command 'java') -and (Test-Path (Join-Path $rootDir 'android' 'gradlew'))) {
        $avail += @{Name='android';Label='Android';Cmd='.\gradlew :app:assembleDebug';Dir=Join-Path $rootDir 'android'}
    }
    if (Test-Command 'pnpm') {
        $avail += @{Name='admin';Label='Admin';Cmd='pnpm run build';Dir=Join-Path $rootDir 'admin'}
    }

    if ($avail.Count -eq 0) {
        Err (T '未检测到编译工具链' 'No build tools found')
        Pause-Menu; Show-MainMenu; return
    }

    Write-Host (T '可编译的端:' 'Buildable ends:') -ForegroundColor Magenta
    foreach ($t in $avail) { Write-Host ('  [ ] ' + $t.Label) }

    $selected = @()
    foreach ($t in $avail) {
        $yn = Confirm-YesNo ('  ' + (T '编译 ' 'Build ') + $t.Label + '?')
        if ($yn) { $selected += $t }
    }
    if ($selected.Count -eq 0) { Pause-Menu; Show-MainMenu; return }

    Step (T '开始并行编译...' 'Starting parallel builds...')
    $logDir = Join-Path $rootDir 'build-logs'
    if (!(Test-Path $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }

    $procs = @()
    foreach ($t in $selected) {
        $logFile  = Join-Path $logDir ($t.Name + '-build.log')
        $exitFile = Join-Path $logDir ($t.Name + '-exit.txt')
        $ps1File  = Join-Path $logDir ($t.Name + '-build.ps1')
        $cmd = $t.Cmd
        $dir = $t.Dir
        $label = $t.Label
        $content = @"
`$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()
`$host.UI.RawUI.WindowTitle = "LinkALL Build: $label"
Write-Host ">>> Building $label..." -ForegroundColor Cyan
`$sw = [Diagnostics.Stopwatch]::StartNew()
try {
    Set-Location "$dir"
    & $cmd 2>&1 | ForEach-Object { Write-Host "`$_" }
    if (`$LASTEXITCODE -ne 0) { throw "exit code `$LASTEXITCODE" }
    `$elapsed = `$sw.Elapsed.TotalSeconds.ToString("F1")
    Write-Host "[OK] $label in `$elapsed s" -ForegroundColor Green
    Set-Content "$exitFile" "0" -Encoding ASCII
} catch {
    Write-Host "[FAIL] ${label}: `$_" -ForegroundColor Red
    Set-Content "$exitFile" "1" -Encoding ASCII
}
"@
        Set-Content $ps1File $content -Encoding UTF8

        Write-Host ('  ' + (T '启动: ' 'Started: ') + $t.Label) -ForegroundColor DarkGray
        $procs += [PSCustomObject]@{
            Name = $t.Name
            Label = $t.Label
            Proc = Start-Process -FilePath 'powershell' -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File', $ps1File -WindowStyle Normal -PassThru
            ExitFile = $exitFile
        }
        Start-Sleep -Milliseconds 800
    }

    Write-Host ''
    Write-Host (T '等待编译完成 (每个任务一个独立窗口, 完成后自动关闭)...' 'Waiting for builds (one window per task, closes when done)...') -ForegroundColor Yellow

    $timeout = 7200000
    $elapsedMs = 0
    while ($elapsedMs -lt $timeout) {
        $remaining = $procs | Where-Object { -not $_.Proc.HasExited }
        if ($remaining.Count -eq 0) { break }
        Start-Sleep -Milliseconds 2000
        $elapsedMs += 2000
        if (($elapsedMs % 10000) -eq 0) {
            Write-Host ('  ' + (T '等待中... ' 'Waiting... ') + $remaining.Count.ToString() + ' ' + (T '任务' 'job(s)')) -ForegroundColor DarkGray
        }
    }
    foreach ($p in $procs) { if (-not $p.Proc.HasExited) { $p.Proc.Kill() } }

    Write-Host ''
    Write-Host (T '编译结果:' 'Build Results:') -ForegroundColor Magenta
    $allOk = $true
    foreach ($p in $procs) {
        if (Test-Path $p.ExitFile) {
            $code = (Get-Content $p.ExitFile -Raw).Trim()
            if ($code -eq '0') {
                Write-Host ('  [OK] ' + $p.Label) -ForegroundColor Green
            } else {
                Write-Host ('  [FAIL] ' + $p.Label + ' - ' + (T '日志: ' 'Log: ') + (Join-Path $logDir ($p.Name + '-build.log'))) -ForegroundColor Red
                $allOk = $false
            }
        } else {
            Write-Host ('  [FAIL] ' + $p.Label + ' - ' + (T '未知错误' 'Unknown error')) -ForegroundColor Red
            $allOk = $false
        }
    }
    if ($allOk) {
        Write-Host ''
        Write-Host (T '所有编译通过!' 'All builds passed!') -ForegroundColor Green
        Write-Host '  server: server/linkall-server'
        Write-Host '  web:    web/dist/'
        Write-Host '  tauri:  controlled-win/src-tauri/target/release/bundle/'
        Write-Host '  android: android/app/build/outputs/apk/debug/app-debug.apk'
        Write-Host '  admin:  admin/dist/'
    }
    Write-Host ('  ' + (T '编译日志: ' 'Build logs: ') + $logDir) -ForegroundColor DarkGray
    Pause-Menu; Show-MainMenu
}

# ========================== GITHUB ACTIONS ==========================
function Invoke-GitHubActions {
    Clear-Host
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host '  GitHub Actions ' + (T '自动部署' 'Auto Deploy') -ForegroundColor Cyan
    Write-Host '========================================' -ForegroundColor Cyan
    Write-Host ''

    $gitDir = Join-Path $rootDir '.git'
    if (!(Test-Path $gitDir)) {
        Err (T '未找到 .git 目录!' '.git not found!')
        $yn = Confirm-YesNo (T '  初始化 git 仓库? (y/N)' '  Init git repo? (y/N)')
        if ($yn) {
            Push-Location $rootDir
            git init
            git add -A
            git commit -m 'Initial commit'
            Pop-Location
            Ok (T 'Git 仓库已初始化' 'Git repo initialized')
        } else { Pause-Menu; Show-MainMenu; return }
    }

    Push-Location $rootDir
    $remoteUrl = git config --get remote.origin.url 2>$null
    Pop-Location

    if ($remoteUrl) {
        Write-Host (T '远程仓库: ' 'Remote: ') -ForegroundColor Magenta -NoNewline; Write-Host $remoteUrl -ForegroundColor White
        $owner = if ($remoteUrl -match '(?:github\.com[:/])([^/]+)/([^/\.]+?)(?:\.git)?$') { $matches[1] } else { $null }
        if ($owner) {
            $msg = T "GitHub 用户/组织: $owner" "GitHub user/org: $owner"
            Ok $msg
        }
    } else {
        Warn (T '未设置 remote.origin.url' 'remote.origin.url not set')
    }

    $yn = Confirm-YesNo (T '  配置/修改远程仓库? (y/N)' '  Configure/change remote? (y/N)')
    if ($yn) {
        $newUrl = Read-Host (T '  输入 GitHub 仓库 URL: ' '  Enter GitHub repo URL: ')
        if ($newUrl) {
            Push-Location $rootDir
            if ($remoteUrl) { git remote set-url origin $newUrl }
            else { git remote add origin $newUrl }
            Pop-Location
            $remoteUrl = $newUrl
        }
    }

    Push-Location $rootDir
    $branch = git rev-parse --abbrev-ref HEAD
    Pop-Location
    Write-Host (T '当前分支: ' 'Current branch: ') -NoNewline; Write-Host $branch -ForegroundColor White
    $yn = Confirm-YesNo (T '  推送到当前分支? (y/N)' '  Push to current branch? (y/N)')
    if (-not $yn) { $branch = Read-Host (T '  输入分支名: ' '  Enter branch name: ') }

    # create workflows if needed
    $wfDir = Join-Path $rootDir '.github' 'workflows'
    if (!(Test-Path $wfDir)) {
        Warn (T '未找到 .github/workflows' '.github/workflows not found')
        $yn = Confirm-YesNo (T '  创建基础 CI workflow? (y/N)' '  Create basic CI workflow? (y/N)')
        if ($yn) {
            New-Item -ItemType Directory -Path $wfDir -Force | Out-Null
            $ciContent = @"
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: cd server && go vet ./... && go build ./...
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: cd web && npm install && npm run build
      - run: cd admin && npm install && npm run build
"@
            Set-Content (Join-Path $wfDir 'ci.yml') $ciContent -Encoding UTF8
            Ok (T 'ci.yml 已创建' 'ci.yml created')
        }
    }

    $yn = Confirm-YesNo (T '  确认提交并推送? (y/N)' '  Confirm commit and push? (y/N)')
    if ($yn) {
        $msg = Read-Host (T '  提交信息: ' '  Commit message: ')
        if (!$msg) { $msg = 'chore: update' }
        Push-Location $rootDir
        git add -A
        git commit -m $msg
        git push origin $branch
        if ($LASTEXITCODE -eq 0) {
            Ok (T '推送成功!' 'Push successful!')
            $yn = Confirm-YesNo (T '  推送 tag 触发 Release? (y/N)' '  Push tag to trigger Release? (y/N)')
            if ($yn) {
                $tag = Read-Host (T '  版本号 (如 v1.0.0): ' '  Version (e.g. v1.0.0): ')
                if ($tag) {
                    git tag $tag
                    git push origin $tag
                    Ok (T 'Tag ' 'Tag ') -NoNewline
                    Write-Host $tag -NoNewline -ForegroundColor White
                    Ok (T ' 已推送' ' pushed')
                }
            }
        } else { Err (T '推送失败' 'Push failed') }
        Pop-Location
    }
    Pause-Menu; Show-MainMenu
}

# ========================== ENTRY ==========================
Ensure-EnvFile

Show-MainMenu
