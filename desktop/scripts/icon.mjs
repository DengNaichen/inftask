import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const desktop = fileURLToPath(new URL('..', import.meta.url));
const temporary = mkdtempSync(path.join(tmpdir(), 'inftask-icon-'));
const run = (command, args) => execFileSync(command, args, { stdio: 'inherit', timeout: 60_000 });
try {
  const png = path.join(temporary, 'icon.png');
  run('/Applications/Xcode.app/Contents/Applications/Icon Composer.app/Contents/Executables/ictool', [
    path.resolve(desktop, '../assets/InfTask.icon'), '--export-image', '--output-file', png,
    '--platform', 'macOS', '--rendition', 'Default', '--width', '1024', '--height', '1024',
    '--scale', '1', '--design-generation', '27',
  ]);
  const iconset = path.join(temporary, 'icon.iconset');
  mkdirSync(iconset);
  for (const size of [16, 32, 128, 256, 512]) {
    for (const scale of [1, 2]) {
      run('sips', ['-z', String(size * scale), String(size * scale), png,
        '--out', path.join(iconset, `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`)]);
    }
  }
  mkdirSync(path.join(desktop, 'assets'), { recursive: true });
  run('iconutil', ['-c', 'icns', iconset, '-o', path.join(desktop, 'assets/icon.icns')]);
} finally {
  rmSync(temporary, { recursive: true });
}
