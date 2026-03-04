function Send-Heartbeat {
    param([string] $Stage, [string] $TaskID)
    $headers = @{
        "X-Lab-TaskID" = $TaskID
        "X-Lab-Stage"  = $Stage
        "X-Lab-Time"   = (Get-Date -Format "HH:mm:ss")
    }
    # This will hit your Go server's listener
    Invoke-RestMethod -Uri "http://verification.net" -Headers $headers -Method Get -ErrorAction SilentlyContinue
}

$TaskID = New-Guid

while ($true) {
    $null = Send-Heartbeat -Stage 'start of heartbeat test' -TaskID $TaskID
    Start-Sleep -Seconds 5
}