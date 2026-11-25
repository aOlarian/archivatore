/* MAP
 * §1 Enterprise edition first-launch notice  L5-L31
 */

// §1 Enterprise edition first-launch notice
'use strict';

const { app, dialog, shell } = require('electron');
const fs = require('node:fs/promises');
const path = require('node:path');

const detail =
  'We build and deploy a paid Enterprise edition to order. Examples of what we can develop and extend for your organization are listed below.\n\n' +
  '• Centralized batch archiving policies, scheduled pipelines and automated cloud or NAS handoff across workstations and servers.\n' +
  '• Managed encryption (AES-256 / GPG), compliance checksum manifests and post-archive integrity verification.\n' +
  '• Silent MSI/GPO deployment, role-based presets and audit logs for IT teams.\n\n' +
  'For an Enterprise edition or custom development, contact us at verobots.eu or olarian.com.';

const sites = ['https://verobots.eu/', 'https://olarian.com/'];
let checking = false;

async function showEnterprise(window) {
  if (checking || window.isDestroyed()) return;
  checking = true;
  const marker = path.join(app.getPath('userData'), 'enterprise-intro-v1');
  try {
    try { await fs.access(marker); return; } catch (err) { if (err.code !== 'ENOENT') throw err; }
    const { response } = await dialog.showMessageBox(window, {
      type: 'info',
      title: 'Archivatore',
      message: 'Enterprise edition',
      detail,
      buttons: ['Continue', 'verobots.eu', 'olarian.com'],
      defaultId: 0,
      cancelId: 0,
    });
    await fs.writeFile(marker, 'seen\n');
    if (sites[response - 1]) await shell.openExternal(sites[response - 1]);
  } catch (err) {
    console.warn('Enterprise introduction:', err.code || 'unavailable');
  } finally {
    checking = false;
  }
}

module.exports = { showEnterprise };
