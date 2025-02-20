local v = importstr '../../VERSION';

{
  namespace: 'default',
  version: std.strReplace(v, '\n', ''),
  image: 'quay.io/rhobs/obo-prometheus-operator:v' + self.version,
  configReloaderImage: 'quay.io/rhobs/obo-prometheus-config-reloader:v' + self.version,
}
