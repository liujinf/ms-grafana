import { PluginSignatureStatus, PluginSignatureType, PluginType } from '@grafana/data';

export const localPlugin = {
  category: '',
  defaultNavUrl: '/plugins/akumuli-datasource/',
  enabled: true,
  hasUpdate: false,
  id: 'akumuli-datasource',
  info: {
    author: {
      name: 'Eugene Lazin',
      url: 'https://akumuli.org',
    },
    description: 'Datasource plugin for Akumuli time-series database',
    links: [
      {
        name: 'Project site',
        url: 'https://github.com/akumuli/Akumuli',
      },
    ],
    logos: {
      small: 'public/plugins/akumuli-datasource/img/logo.svg.png',
      large: 'public/plugins/akumuli-datasource/img/logo.svg.png',
    },
    build: {},
    screenshots: null,
    version: '1.3.12',
    updated: '2019-12-19',
  },
  latestVersion: '1.3.12',
  name: 'Akumuli',
  pinned: false,
  signature: PluginSignatureStatus.valid,
  signatureOrg: 'Grafana Labs',
  signatureType: 'community',
  state: '',
  type: PluginType.datasource,
};

export const remotePlugin = {
  createdAt: '2016-04-06T20:23:41.000Z',
  description: 'Zabbix plugin for Grafana',
  downloads: 33645089,
  downloadSlug: 'alexanderzobnin-zabbix-app',
  featured: 180,
  id: 74,
  internal: false,
  json: {
    dependencies: {
      grafanaDependency: '>=7.3.0',
      grafanaVersion: '7.3',
    },
    info: {
      links: [],
    },
  },
  links: [],
  name: 'Zabbix',
  orgId: 13056,
  orgName: 'Alexander Zobnin',
  orgSlug: 'alexanderzobnin',
  orgUrl: 'https://github.com/alexanderzobnin',
  packages: {},
  popularity: 0.2111,
  readme:
    '<h1>Zabbix plugin for Grafana</h1>\n<p>:copyright: 2015-2021 Alexander Zobnin alexanderzobnin@gmail.com</p>\n<p>Licensed under the Apache 2.0 License</p>',
  signatureType: PluginSignatureType.community,
  slug: 'alexanderzobnin-zabbix-app',
  status: 'active',
  typeCode: PluginType.app,
  typeId: 1,
  typeName: 'Application',
  updatedAt: '2021-05-18T14:53:01.000Z',
  url: 'https://github.com/alexanderzobnin/grafana-zabbix',
  userId: 0,
  verified: false,
  version: '4.1.5',
  versionSignatureType: PluginSignatureType.community,
  versionSignedByOrg: 'alexanderzobnin',
  versionSignedByOrgName: 'Alexander Zobnin',
  versionStatus: 'active',
};
