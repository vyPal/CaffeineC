Register-ArgumentCompleter -Native -CommandName 'CaffeineC' -ScriptBlock {
     param($commandName, $wordToComplete, $cursorPosition)
     $other = "$wordToComplete --generate-shell-completion"
         Invoke-Expression $other | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
         }
 }