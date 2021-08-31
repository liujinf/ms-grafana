import { remotePlugin, localPlugin } from './mocks';
import { mapToCatalogPlugin, mapRemoteToCatalog, mapLocalToCatalog } from './helpers';
import { RemotePlugin, LocalPlugin } from './types';
import { PluginSignatureStatus, PluginSignatureType } from '../../../../../packages/grafana-data/src';

describe('Plugins/Helpers', () => {
  describe('mapRemoteToCatalog()', () => {
    test('maps remote response to PluginCatalog', () => {
      expect(mapRemoteToCatalog(remotePlugin)).toEqual({
        description: 'Zabbix plugin for Grafana',
        downloads: 33645089,
        hasUpdate: false,
        id: 'alexanderzobnin-zabbix-app',
        info: {
          logos: {
            large: 'https://grafana.com/api/plugins/alexanderzobnin-zabbix-app/versions/4.1.5/logos/large',
            small: 'https://grafana.com/api/plugins/alexanderzobnin-zabbix-app/versions/4.1.5/logos/small',
          },
        },
        isCore: false,
        isDev: false,
        isEnterprise: false,
        isInstalled: false,
        name: 'Zabbix',
        orgName: 'Alexander Zobnin',
        popularity: 0.2111,
        publishedAt: '2016-04-06T20:23:41.000Z',
        signature: 'valid',
        type: 'app',
        updatedAt: '2021-05-18T14:53:01.000Z',
        version: '4.1.5',
      });
    });

    test('adds the correct signature enum', () => {
      const pluginWithoutSignature = { ...remotePlugin, signatureType: '', versionSignatureType: '' } as RemotePlugin;
      // With only "signatureType" -> valid
      const pluginWithSignature1 = { ...remotePlugin, signatureType: PluginSignatureType.commercial } as RemotePlugin;
      // With only "versionSignatureType" -> valid
      const pluginWithSignature2 = { ...remotePlugin, versionSignatureType: PluginSignatureType.core } as RemotePlugin;

      expect(mapRemoteToCatalog(pluginWithoutSignature).signature).toBe(PluginSignatureStatus.missing);
      expect(mapRemoteToCatalog(pluginWithSignature1).signature).toBe(PluginSignatureStatus.valid);
      expect(mapRemoteToCatalog(pluginWithSignature2).signature).toBe(PluginSignatureStatus.valid);
    });

    test('adds an "isEnterprise" field', () => {
      const enterprisePlugin = { ...remotePlugin, status: 'enterprise' } as RemotePlugin;
      const notEnterprisePlugin = { ...remotePlugin, status: 'unknown' } as RemotePlugin;

      expect(mapRemoteToCatalog(enterprisePlugin).isEnterprise).toBe(true);
      expect(mapRemoteToCatalog(notEnterprisePlugin).isEnterprise).toBe(false);
    });

    test('adds an "isCore" field', () => {
      const corePlugin = { ...remotePlugin, internal: true } as RemotePlugin;
      const notCorePlugin = { ...remotePlugin, internal: false } as RemotePlugin;

      expect(mapRemoteToCatalog(corePlugin).isCore).toBe(true);
      expect(mapRemoteToCatalog(notCorePlugin).isCore).toBe(false);
    });
  });

  describe('mapLocalToCatalog()', () => {
    test('maps local response to PluginCatalog', () => {
      expect(mapLocalToCatalog(localPlugin)).toEqual({
        description: 'Datasource plugin for Akumuli time-series database',
        downloads: 0,
        id: 'akumuli-datasource',
        info: {
          logos: {
            small: 'public/plugins/akumuli-datasource/img/logo.svg.png',
            large: 'public/plugins/akumuli-datasource/img/logo.svg.png',
          },
        },
        name: 'Akumuli',
        orgName: 'Eugene Lazin',
        popularity: 0,
        publishedAt: '',
        signature: 'valid',
        updatedAt: '2019-12-19',
        version: '1.3.12',
        hasUpdate: false,
        isInstalled: true,
        isCore: false,
        isDev: false,
        isEnterprise: false,
        type: 'datasource',
      });
    });

    test('isCore if signature is internal', () => {
      const pluginWithoutInternalSignature = { ...localPlugin };
      const pluginWithInternalSignature = { ...localPlugin, signature: 'internal' } as LocalPlugin;
      expect(mapLocalToCatalog(pluginWithoutInternalSignature).isCore).toBe(false);
      expect(mapLocalToCatalog(pluginWithInternalSignature).isCore).toBe(true);
    });

    test('isDev if local.dev', () => {
      const pluginWithoutDev = { ...localPlugin, dev: false };
      const pluginWithDev = { ...localPlugin, dev: true };
      expect(mapLocalToCatalog(pluginWithoutDev).isDev).toBe(false);
      expect(mapLocalToCatalog(pluginWithDev).isDev).toBe(true);
    });
  });

  describe('mapToCatalogPlugin()', () => {
    test('levi please name this later', () => {
      expect(mapToCatalogPlugin(localPlugin, remotePlugin)).toEqual({
        description: 'Zabbix plugin for Grafana',
        downloads: 33645089,
        hasUpdate: true,
        id: 'alexanderzobnin-zabbix-app',
        info: {
          logos: {
            small: 'https://grafana.com/api/plugins/alexanderzobnin-zabbix-app/versions/4.1.5/logos/small',
            large: 'https://grafana.com/api/plugins/alexanderzobnin-zabbix-app/versions/4.1.5/logos/large',
          },
        },
        isCore: false,
        isDev: false,
        isEnterprise: false,
        isInstalled: true,
        name: 'Zabbix',
        orgName: 'Alexander Zobnin',
        popularity: 0.2111,
        publishedAt: '2016-04-06T20:23:41.000Z',
        type: 'app',
        signature: 'valid',
        updatedAt: '2021-05-18T14:53:01.000Z',
        version: '4.1.5',
      });
    });

    test('jacks playground', () => {
      const withoutLocalVersion = [
        { ...localPlugin, info: { ...localPlugin.info, version: '1.2.1' } },
        { ...remotePlugin },
      ];
      const [local, remote] = withoutLocalVersion;
      // const withoutRemoteVersion;
      // const withoutAnyVersion;
      expect(mapToCatalogPlugin(local, remote));
    });

    test('prefers the remote version for the description', () => {
      //   expect(mapToCata);
    });
  });
});
