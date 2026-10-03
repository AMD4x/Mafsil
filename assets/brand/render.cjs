// Render vector originals only. This never builds the application or packages.
// Requires Node.js and sharp. See README.md in this directory.
const fs = require('node:fs');
const path = require('node:path');
const sharp = require('sharp');
const root = __dirname;

async function main() {
  const exports = [
    ['mark-master', 4096], ['logo-master', 4096],
    ['logo-light', 2048], ['logo-dark', 2048],
    ['icon', 1024], ['icon-dark', 1024], ['icon-mono', 1024],
    ['social', 1280],
    ['../architecture', 2560], ['../architecture-mobile', 800],
  ];
  for (const [name, width] of exports) {
    await sharp(path.join(root, name+'.svg'), {density: 288})
      .resize({width}).png().toFile(path.join(root, name+'.png'));
  }
  await sharp(path.join(root, 'social.svg'), {density: 288})
    .resize({width: 2560}).png().toFile(path.join(root, 'social-hd.png'));
  const frames = [];
  for (const size of [16, 24, 32, 48, 64, 128, 256]) {
    const source = size <= 24 ? `icon-${size}.svg` : 'icon.svg';
    const data = await sharp(path.join(root, source), {density: size <= 24 ? 72 : 288})
      .resize(size, size).png().toBuffer();
    frames.push({size, data});
  }
  const header = Buffer.alloc(6+16*frames.length);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(frames.length, 4);
  let offset = header.length;
  frames.forEach(({size, data}, i) => {
    const at = 6+i*16;
    header[at] = header[at+1] = size === 256 ? 0 : size;
    header.writeUInt16LE(1, at+4);
    header.writeUInt16LE(32, at+6);
    header.writeUInt32LE(data.length, at+8);
    header.writeUInt32LE(offset, at+12);
    offset += data.length;
  });
  fs.writeFileSync(path.join(root, 'mafsil.ico'), Buffer.concat([header, ...frames.map(x=>x.data)]));
  console.log('Rendered brand artwork and ICO frames only; no application build.');
}
main().catch(error => { console.error(error); process.exitCode = 1; });
