"""Offline regressions. SSH, process termination, health, and builds are mocks."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
sys.dont_write_bytecode = True
import tempfile
import types
import unittest
from unittest.mock import MagicMock, patch

ROOT = Path(__file__).resolve().parents[1]

class ToolingSafety(unittest.TestCase):
    def test_ssh_uses_external_configuration_and_rejects_unknown_hosts(self):
        client = MagicMock()
        paramiko = types.SimpleNamespace(SSHClient=lambda: client, RejectPolicy=lambda: 'reject')
        spec = importlib.util.spec_from_file_location('fixture_ssh', ROOT/'scratch/ssh_config.py')
        module = importlib.util.module_from_spec(spec)
        with patch.dict(sys.modules, {'paramiko':paramiko}): spec.loader.exec_module(module)
        client.connect.assert_not_called()
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(ValueError): module.connect()
        client.connect.assert_not_called()
        with patch.dict(os.environ, {'OVH_DEPLOY_HOST':'fixture.invalid','OVH_DEPLOY_USER':'fixture'}, clear=True):
            self.assertIs(module.connect(),client)
        client.set_missing_host_key_policy.assert_called_with('reject')
        self.assertIsNone(client.connect.call_args.kwargs['password'])
        self.assertEqual(client.connect.call_args.args,('fixture.invalid',))
        client.connect.side_effect = RuntimeError('fixture failure')
        with patch.dict(os.environ, {'OVH_DEPLOY_HOST':'fixture.invalid','OVH_DEPLOY_USER':'fixture'}, clear=True):
            with self.assertRaises(RuntimeError): module.connect()
        client.close.assert_called_once()

    def test_rebuild_control_flow_without_process_or_build_side_effects(self):
        source=(ROOT/'scripts/start-backend.ps1').read_text(encoding='utf-8-sig')
        source=source[:source.index('# Fully detached:')]
        source=source.replace('param(\n  [switch]$Rebuild,\n  [switch]$Stop\n)', '$Rebuild = $env:FIXTURE_REBUILD -eq "1"; $Stop = $false')
        prefix = r'''
function Invoke-WebRequest { return [pscustomobject]@{StatusCode=200} }
function Get-Process { return @([pscustomobject]@{Path=$Exe},[pscustomobject]@{Path="C:\other-workspace\ovh-webui.exe"}) }
function Stop-Process { param([Parameter(ValueFromPipeline=$true)]$InputObject,[switch]$Force); process { Add-Content -LiteralPath (Join-Path $Backend "stopped.txt") -Value $InputObject.Path } }
function Start-Sleep { }
function go { $BuildArgs = $args
 if ($env:FIXTURE_BUILD_FAIL -eq "1") { $global:LASTEXITCODE=1; return }
 Set-Content -LiteralPath $BuildArgs[2] -Value "new-binary"
 $global:LASTEXITCODE=0
}
'''
        for rebuild, fail in [(False,False),(True,True),(True,False)]:
            with self.subTest(rebuild=rebuild,fail=fail), tempfile.TemporaryDirectory() as temp:
                root=Path(temp); (root/'scripts').mkdir(); (root/'backend').mkdir()
                binary=root/'backend/ovh-webui.exe'; binary.write_text('old-binary')
                (root/'backend/.env').write_text('# fixture')
                script=root/'scripts/start.ps1'; script.write_text(prefix+source,encoding='utf-8-sig')
                env={**os.environ,'FIXTURE_REBUILD':str(int(rebuild)),'FIXTURE_BUILD_FAIL':str(int(fail))}
                # PowerShell 按系统代码页输出(中文 Windows 是 GBK),按 UTF-8 严格解码会让读线程抛错、输出丢失
                result=subprocess.run(['powershell','-NoProfile','-ExecutionPolicy','Bypass','-File',str(script)],env=env,capture_output=True,text=True,encoding='utf-8',errors='replace')
                self.assertEqual(result.returncode,1 if fail else 0,result.stderr)
                self.assertEqual(binary.read_text().strip(),'new-binary' if rebuild and not fail else 'old-binary')
                stopped=root/'backend/stopped.txt'
                if not rebuild or fail: self.assertFalse(stopped.exists())
                else: self.assertTrue(all(line.lower()==str(binary).lower() for line in stopped.read_text().splitlines()))

if __name__ == '__main__': unittest.main()
