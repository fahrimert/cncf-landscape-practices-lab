# CNCF Landscape Practices Lab 

Bu depo, Cloud Native Computing Foundation  ekosistemindeki en güçlü ve modern araçları bir araya getirerek, Production-Grade bir Kubernetes platform mimarisi inşa etme laboratuvarıdır.

Amacımız, standart ve basit kurulumların ötesine geçerek; ağ , depolama , güvenlik , gözlemlenebilirlik ve uygulama teslimatı süreçlerini sektör standartlarında, birbirleriyle entegre bir şekilde tasarlamak ve test etmektir.

## Katmanlı İnşaat Yol Haritası

Bu devasa mimariyi yönetilebilir kılmak ve hata tespitini kolaylaştırmak için sistem Katmanlar halinde inşa edilmektedir.

### Katman 0 Temel Atma 

Sektörde sıkça yapılan bir hata, Kubernetes'i varsayılan ağ eklentileriyle kurup sonradan üzerine gelişmiş araçlar  eklemeye çalışmaktır. Bu laboratuvarda biz sıfır noktasından başlıyoruz.

* **Yıkım :** Var olan K3s kurulumu, geride hiçbir ağ veya yapılandırma artığı kalmayacak şekilde tamamen silinir (`ops/playbooks/reset-and-rebuild.yaml`).
* **Çıplak Kurulum:** K3s; varsayılan CNI, LoadBalancer, Ingress ve Network Policy bileşenleri tamamen kapatılarak (`--flannel-backend=none`, vb.) kurulur.
* **Sonuç:** Bu aşamada Cluster ayağa kalkar ancak ağ bileşeni olmadığı için Node'lar bilerek `NotReady` durumunda bekletilir.

### Katman 1: eBPF Tabanlı Sinir Sistemi

"Çıplak" K3s üzerine, Linux çekirdeğinin gücünü eBPF teknolojisiyle kullanan Cilium entegre edilir.

* **Infrastructure as Code (IaC):** Cilium kurulumu, projenin `stacks/network-cilium` dizinindeki Helm tanımlamalarıyla (`Chart.yaml`, `values.yaml`) gerçekleştirilir.
* **Sonuç:** Cilium kurulduğu an `NotReady` durumundaki K3s Node'ları `Ready` durumuna geçer ve cluster'ın iletişim otoyolu (Data Plane) açılmış olur.

## Başlangıç Rehberi (Getting Started)

Laboratuvarı yerel ortamınızda ayağa kaldırmak için aşağıdaki adımları sırasıyla uygulayın:

### 1. K3s Temizliği ve Çıplak Kurulum (Layer 0)

K3s'i ağ bileşenleri olmadan kurmak için hazırlanan Ansible playbook'unu çalıştırın:

```bash
ansible-playbook -i playbooks/inventory.ini playbooks/stack-1-custom-playbooks/reset-and-rebuild.yaml
```

Not: İşlem sonrasında kubectl get nodes komutu NotReady statüsü vermelidir.

### Katman 2: Bulut Yerel Depolama  Rook & Ceph

Kubernetes üzerinde durum bilgisi tutan uygulamalar ve veritabanları çalıştırabilmek için sağlam bir depolama altyapısına ihtiyaç vardır. Bu laboratuvarda, endüstri standardı olan dağıtık depolama sistemi Ceph'i, Kubernetes'e özgü bir operatör olan Rook ile yönetiyoruz.

**KRİTİK ÖN KOŞUL :**
Ceph ağır bir sistemdir ve veriyi güvenle yazabilmek için gerçek, ham bir diske ihtiyaç duyar. Kuruluma geçmeden önce `Vagrantfile` dosyanızı aşağıdaki gibi güncelleyerek makineye en az 8GB RAM, 4 CPU ve 10GB'lık ekstra bir ham disk eklediğinizden emin olun:

```ruby
Vagrant.configure("2") do |config|
  config.vm.disk :disk, size: "10GB", name: "ceph_osd_disk"
  config.vm.provider "virtualbox" do |vb|
    vb.memory = "8192"
    vb.cpus = "4"
  end
end
```
Değişiklik sonrası vagrant reload veya vagrant up ile makineyi başlatın.

### 1. Şantiye Şefinin (Rook Operator) Kurulması

Önce Ceph kümesini yönetecek olan operatörü kuruyoruz.

```bash
KUBECONFIG=./k3s.yaml  helm dependency update ./stacks/stack-1/storage-rook/
```

```bash
KUBECONFIG=./k3s.yaml  helm install rook-operator ./stacks/stack-1/storage-rook/ \
  --namespace rook-ceph \
  --create-namespace
```

**Not:** `KUBECONFIG=./k3s.yaml  kubectl get pods -n rook-ceph` ile `rook-ceph-operator` podunun `Running` olmasını bekleyin.

### 2. Ceph Cluster'ın İnşası

Laboratuvar ortamını yormamak adına gereksiz dosya sistemlerini kapattığımız ve yeni eklenen ham diski otomatik bulan, tek node'a optimize edilmiş `stacks/stack-1/storage-rook-cluster/values.yaml` yapılandırmasını kuruyoruz:

## Kurulumu Başlatın
```bash
KUBECONFIG=./k3s.yaml helm dependency update ./stacks/stack-1/storage-rook-cluster/
```

```bash
KUBECONFIG=./k3s.yaml helm install rook-cluster ./stacks/stack-1/storage-rook-cluster/ --namespace rook-ceph
```

### 3. Doğrulama

Kurulum tetiklendikten sonra sistem sırasıyla disk bağlayıcıları (csi), gözcüleri (mon), yöneticileri (mgr) ve en son veriyi yazacak işçiyi (osd) ayağa kaldırır.

Durumu canlı izlemek için:

```bash
KUBECONFIG=./k3s.yaml kubectl get pods -n rook-ceph -w
```
Listede rook-ceph-osd-0-... isimli podun Running durumuna geçtiğini gördüğünüzde kurulum başarıyla tamamlanmış demektir.

```bash
KUBECONFIG=./k3s.yaml kubectl exec -it deploy/rook-ceph-tools -n rook-ceph -- ceph status
# Çıktıda "osd: 1 osds: 1 up, 1 in" ibaresi diskin sağlıklı entegre olduğunu gösterir.
```

### Katman 3: Dağıtık Veritabanı (Cloud Native Database - Vitess)

Projemizin kalıcı veri (stateful) ihtiyacı için, YouTube ve Slack gibi devlerin kullandığı, Kubernetes üzerinde yatayda sınırsız ölçeklenebilen Vitess veritabanını kullanıyoruz. Vitess, verilerini bir önceki katmanda kurduğumuz Rook-Ceph blok disklerine yazacak şekilde entegre edilmiştir.

**Önemli Not:** Local K3s  ortamındaki kaynak kısıtlamalarına (Insufficient CPU hatalarına) takılmamak için, Vitess bileşenlerinin varsayılan CPU talepleri (`requests: cpu: 100m`), kağıt üzerinde sistemi boğmaması adına `10m` olarak optimize edilmiştir.



### 1. Optimize Edilmiş Veritabanı Kümesi  Konfigürasyonu

Buradaki `stacks/stack-1/database-vitess/101_initial_cluster.yaml` bu dosya, CPU tüketimi laboratuvar ortamına göre optimize edilmiş ve `rook-ceph-block` disk sağlayıcısına bağlanmış hazır versiyondur.

### 2. Kurulumu Başlatma

Konfigürasyonları Kubernetes kümesine uyguluyoruz:

```bash
KUBECONFIG=./k3s.yaml kubectl create namespace vitess
```

```bash
KUBECONFIG=./k3s.yaml kubectl apply -f stacks/stack-1/database-vitess/operator.yaml -n vitess
```

```bash
KUBECONFIG=./k3s.yaml kubectl apply -f stacks/stack-1/database-vitess/101_initial_cluster.yaml -n vitess
```

Podların ayağa kalkmasını izlemek için:

```bash
KUBECONFIG=./k3s.yaml kubectl get pods -n vitess -w
```

### Katman 4: Service Mesh ve Ağ Güvenliği 

Mikroservis mimarimizdeki uygulamaların (Order, Inventory, Payment vb.) birbirleriyle olan trafiğini (HTTP/gRPC) yönetmek, iletişimi otomatik olarak mTLS ile şifrelemek (Zero-Trust) ve dış dünyadan gelen istekleri güvenle içeri almak için Service Mesh olarak Istio kullanıyoruz.

Aşağıdaki adımları sırasıyla uygulayarak Istio Control Plane ve Ingress Gateway bileşenlerini K3s kümenize sorunsuzca kurabilirsiniz.

#### 1. Istio Helm Deposunun Eklenmesi

Öncelikle Istio'nun resmi Helm paketlerini sisteme tanıtın:

```bash
KUBECONFIG=./k3s.yaml helm repo add istio [https://istio-release.storage.googleapis.com/charts](https://istio-release.storage.googleapis.com/charts)
```

```bash
KUBECONFIG=./k3s.yaml helm repo update
```

### 2. Istio Control Plane Kurulumu

Istio'nun temel kaynaklarını (CRD) ve asıl yönetici beyni olan `istiod` bileşenini izole bir alana (namespace) kuruyoruz:

```bash
KUBECONFIG=./k3s.yaml kubectl create namespace istio-system
```

```bash
KUBECONFIG=./k3s.yaml helm install istio-base istio/base -n istio-system --set defaultRevision=default
```

```bash
KUBECONFIG=./k3s.yaml helm install istiod istio/istiod -n istio-system --wait
```

### 4. Kurulumun Doğrulanması

Komutlar tamamlandıktan sonra, Istio'nun hem beyninin hem de kapısının sağlıklı bir şekilde ayağa kalktığını kontrol edin:

```bash
KUBECONFIG=./k3s.yaml kubectl get pods -A | grep istio
```

Çıktıda istiod ve istio-ingressgateway podlarının Running durumunda olduğunu görmelisiniz. Bu aşamadan sonra altyapınız, üzerine kurulacak olan mikroservislere otomatik olarak Envoy "Sidecar" enjekte etmeye tamamen hazırdır.


### Katman 5: Modern Gözlemlenebilirlik (Observability - VictoriaMetrics)

Sistemin "MR" cihazı olarak, standart Prometheus altyapısından çok daha az kaynak tüketen ve daha yüksek performans sunan VictoriaMetrics kullanıyoruz. Bu bileşen, topladığı metrikleri kalıcı olarak saklamak için Rook-Ceph blok depolama birimini kullanacak şekilde yapılandırılmıştır.


```bash
KUBECONFIG=./k3s.yaml helm repo add vm [https://victoriametrics.github.io/helm-charts](https://victoriametrics.github.io/helm-charts)
```

### 2. Düşük Kaynaklı ve Ceph Uyumlu Yapılandırma

Laboratuvar ortamındaki CPU/RAM kullanımını minimize eden ve verileri Ceph üzerinde saklayan `custom-values.yaml` dosyasını burada  görebilirsiniz  `stacks/stack-1/observability-victoriametrics/custom-values.yaml`.

### 3. Kurulumun Başlatılması

Gözlem araçları için izole bir alan oluşturup kurulumu gerçekleştiriyoruz:

```bash
KUBECONFIG=./k3s.yaml kubectl create namespace observability
```

```bash
KUBECONFIG=./k3s.yaml helm install vmetrics ./victoria-metrics-single -n observability -f stacks/stack-1/observability-victoriametrics/custom-values.yaml --wait
```

### 4. Kurulumun Doğrulanması

Kurulumun ardından podun ve disk bağlantısının durumunu kontrol edin:

```bash
KUBECONFIG=./k3s.yaml kubectl get pods,pvc -n observability
```

Ekranda podun Running statüsünde olduğunu ve PVC'nin Bound (Rook-Ceph diskinin başarıyla bağlandığı) durumuna geçtiğini görmelisiniz. Artık altyapınız yüksek performanslı metrik toplama kapasitesine sahiptir.


### Katman 6: Merkezi Log Toplama (Observability - Fluentd)

Sistemdeki tüm K3s bileşenlerinin ve mikroservislerin ürettiği logları merkezi bir noktada toplamak, filtrelemek ve iletmek için bir log toplama ajanı olan Fluentd kullanıyoruz. Bu bileşen, her sunucuda bir adet çalışacak şekilde bir **DaemonSet** olarak yapılandırılmıştır.

# Resmi depoyu ekleyin ve chart'ı yerel dizine indirin
```bash
KUBECONFIG=./k3s.yaml helm repo add fluent [https://fluent.github.io/helm-charts](https://fluent.github.io/helm-charts)
```

### 2. Düşük Kaynaklı Yapılandırma

Laboratuvar ortamındaki yoğun "kaynak rezervasyonu" (CPU/RAM requests) kısıtlamalarını aşmak için, Fluentd'nin kağıt üzerindeki taleplerini minimize eden `custom-values.yaml` dosyasını `stacks/stack-1/observability-fluentd/custom-values.yaml` burada görebilirsiniz


### 3. Kurulumun Başlatılması

Fluentd'yi mevcut observability namespace'i altına kuruyoruz:
```bash
KUBECONFIG=./k3s.yaml helm install fluentd ./fluentd -n observability -f stacks/stack-1/observability-fluentd/custom-values.yaml --wait
```

### Katman 7: Dağıtık İzleme (Distributed Tracing - OpenTelemetry & Jaeger)

Mikroservis mimarilerinde bir isteğin (request) servisler arasındaki yolculuğunu uçtan uca izlemek, nerede yavaşladığını veya hata verdiğini anında tespit etmek için dağıtık izleme (tracing) kullanılır. Bu işlem için endüstri standardı olan **OpenTelemetry**  ve **Jaeger** araçlarını birlikte kullanıyoruz.

**Önemli Not:** Local K3s laboratuvar ortamında zamanlayıcının `Insufficient Memory` hatalarını aşmak için, her iki aracın da CPU ve RAM requests  değerleri sembolik seviyelere çekilerek optimize edilmiştir.

### 1. İzleme Klasörünün ve Chart'ların Hazırlanması

Resmi Helm depolarını ekleyip her iki aracın dosyalarını yerel IaC dizinimize indiriyoruz:

```bash
KUBECONFIG=./k3s.yaml helm repo add jaegertracing [https://jaegertracing.github.io/helm-charts](https://jaegertracing.github.io/helm-charts)
KUBECONFIG=./k3s.yaml helm repo add open-telemetry [https://open-telemetry.github.io/opentelemetry-helm-charts](https://open-telemetry.github.io/opentelemetry-helm-charts)
KUBECONFIG=./k3s.yaml helm repo update
```

### 2. Düşük Kaynaklı Yapılandırma Dosyaları
stacks/stack-1/observability-tracing/jaeger-values.yaml ve stacks/stack-1/observability-tracing/otel-values.yaml dosyalarına ihtiyacımız var doğru şekilde yapılandırmak için. 

### 3. Kurulumun Başlatılması
```bash
KUBECONFIG=./k3s.yaml helm install jaeger stacks/stack-1/observability-tracing/jaeger -n observability -f stacks/stack-1/observability-tracing/jaeger-values.yaml --wait

KUBECONFIG=./k3s.yaml helm install otel-collector stacks/stack-1/observability-tracing/opentelemetry-collector -n observability -f stacks/stack-1/observability-tracing/otel-values.yaml --wait
```

### 4. Kurulumun Doğrulanması
```bash
KUBECONFIG=./k3s.yaml kubectl get pods -n observability | grep -E "jaeger|otel"
```
Ekranda podların Running durumunda olduğunu görmelisiniz. Bu aşamadan sonra sisteminiz mikroservisler arası trafiği şeffaf bir şekilde izleme (tracing) yeteneğine kavuşmuştur.

## Katman 8: Çekirdek Seviyesi Güvenlik (Security - Falco)

Sistemimizin en derin noktasında, Linux çekirdeği seviyesinde anormallikleri tespit etmek için CNCF'nin endüstri standardı güvenlik aracı olan **Falco**'yu kullanıyoruz. 

Falco, **eBPF** teknolojisi ile sistem çağrılarını dinleyerek aşağıdaki gibi kural dışı hareketleri anında tespit eder:

* Bir mikroservis podu içinde izinsiz terminal (shell) açılması
* `/etc/shadow` gibi hassas sistem dosyalarına yetkisiz erişim sağlanması

> **Önemli Not:** > Local **K3s** laboratuvar ortamında `Insufficient Memory` hatalarına takılmamak adına, diğer araçlarda olduğu gibi Falco'nun da kaynak talepleri (*requests*) optimize edilmiştir. Ayrıca, kernel ile haberleşmesi için en hafif ve güncel yöntem olan `modern_bpf` aktif hale getirilmiştir.

### 1. Falco Klasörünün ve Chart'ının Hazırlanması

Güvenlik katmanı için IaC dizinini oluşturup resmi paketleri indiriyoruz:

```bash
KUBECONFIG=./k3s.yaml helm repo add falcosecurity [https://falcosecurity.github.io/charts](https://falcosecurity.github.io/charts)
```
### 2. Kurulumun Başlatılması

# Güvenlik için ayrı bir namespace oluşturun
```bash
KUBECONFIG=./k3s.yaml kubectl create namespace security
```

# Falco'yu yerel dizindeki chart ve optimize edilmiş ayarlarımızla kurun
```bash
KUBECONFIG=./k3s.yaml helm install falco stacks/stack-1/security-falco/falco -n security -f stacks/stack-1/security-falco/custom-values.yaml
```
## Katman 9: Uygulama (Dapr, OpenFeature ve Mikroservisler)
Katman 9 da Dapr kurulumu ve redis kurulumu kısmının readmesini atladım.

## Katman 10: GitOps Otoyolu (Continuous Delivery - Flux)

Şimdiye kadar tüm altyapı bileşenlerini terminalden manuel komutlarla (`helm install`, `kubectl apply`) kurduk. Ancak modern CNCF mimarilerinde sistem kendi kendini Git üzerinden güncellemeli ve yönetmelidir. Bu "GitOps" devrimi için sektör lideri olan **Flux**'ı kullanıyoruz.


Aşağıdaki adımlarla K3s kümenizi GitHub deponuza (*repository*) bağlayıp, sistemin otonom hale gelmesini sağlayabilirsiniz:

### 1. Flux CLI Kurulumu

Bilgisayarınıza Flux komut satırı aracını kurun:

```bash
curl -s [https://fluxcd.io/install.sh](https://fluxcd.io/install.sh) | sudo bash
```

### 2. K3s Kümesini GitHub'a Bağlama (Bootstrap)

Öncelikle GitHub'da repo yetkisine sahip bir **Personal Access Token (PAT)** oluşturun ve bunu terminale aktarın. Sonrasında Flux'ı deponuza bağlayın:

```bash
export GITHUB_TOKEN="ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

KUBECONFIG=./k3s.yaml flux bootstrap github \
  --owner=<GITHUB-KULLANICI-ADINIZ> \
  --repository=cncf-landscape-practices \
  --branch=main \
  --path=./clusters/k3s-lab \
  --personal
```
### 3. Altyapı Klasörünü (stack-1) Flux'a Tanıtma

Flux'ın oluşturduğu klasörü bilgisayarınıza çekin. Ardından, mevcut stacks/stack-1 klasöründeki manifestoları yönetecek olan infrastructure.yaml Kustomization dosyasını görebilirsiniz clusters/k3s-lab/flux-system/infrastructure.yaml  burada .Flux'ın ana okuma listesine dahil ettiğini  clusters/k3s-lab/flux-system/kustomization.yaml en altındaki infrastructure.yaml dan görebilirsiniz.

### 4. Güvenli Liste (Whitelist) Oluşturma ve Devrimi Başlatma

Flux'ın stacks/stack-1 içindeki tüm dosyalara (Helm values gibi) müdahale edip çökmesini engellemek için, sadece onaylı manifestoları içeren bir güvenli listeyi stacks/stack-1/kustomization.yaml burada görebilirsiniz.

```bash
git add .
git commit -m "feat(gitops): initialize flux and map stack-1 infrastructure"
git push
```
Artık K3s kümeniz, stacks/stack-1/kustomization.yaml dosyasına eklediğiniz her şeyi Git üzerinden otomatik olarak kuracaktır.



## Katman 11: Webhook ve Sertifika Yönetimi (Cert-Manager)

OpenFeature gibi gelişmiş Kubernetes operatörleri, podların içine ajan enjekte edebilmek için "Mutating Admission Webhook" kullanırlar. Bu webhook'ların Kubernetes API sunucusuyla güvenli  haberleşebilmesi için sisteme otomatik sertifika üretecek olan endüstri standardı **cert-manager** bileşenini GitOps üzerinden kuruyoruz.


### 1. Cert-Manager GitOps Manifestosunu Hazırlama

CRD'leri (*Custom Resource Definitions*) otomatik kuracak şekilde yapılandırılmış manifestoyu görebilirsiniz `stacks/stack-1/security-cert-manager/cert-manager.yaml` bu dosyada.


### 2. Kurulum

Bu dosyayı `stacks/stack-1/kustomization.yaml` listesine ekleyip Git'e gönderin. Flux kurulumu tamamladığında podların ayakta olduğunu doğrulayın:

```bash
kubectl get pods -n cert-manager
# Üç podun da (cert-manager, cainjector, webhook) Running durumunda olduğundan emin olun.
```

## Katman 12: Dinamik Özellik Yönetimi (Feature Flagging - OpenFeature)

Uygulama kodunu değiştirmeden, yeni bir Docker imajı almadan ve K3s podlarını yeniden başlatmadan (sıfır kesinti) yeni özellikleri canlı yayında açıp kapatabilmek için **OpenFeature** ve **flagd** kullanıyoruz.

### 1. OpenFeature Operatörü Manifestosu

Operatörü kurmak için şuradaki dosyayı görebilirsiniz  `stacks/stack-1/feature-flagging-openfeature/openfeature.yaml`

### 2. Bayrakları (Flags) ve Kaynak Haritasını Tanımlama

Uygulamalarımızın kullanacağı özellikleri (`fast-delivery` vb.) tanımlayan CRD dosyalarını görebilirsiniz şuralarda `stacks/stack-1/feature-flagging-openfeature/flags.yaml`ve `stacks/stack-1/feature-flagging-openfeature/source.yaml`.

### 3. Güvenli Listeye Ekleme ve Otonom Kurulum

Tüm bu bileşenleri Flux'ın izlediği `stacks/stack-1/kustomization.yaml` dosyasına ekleyin. Dosyanın son hali şu şekilde olmalıdır:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - application-gateway/ingress.yaml
  - container-registry-dragonfly/dragonfly.yaml
  - security-cert-manager/cert-manager.yaml
  - feature-flagging-openfeature/openfeature.yaml
  - feature-flagging-openfeature/flags.yaml
  - feature-flagging-openfeature/source.yaml
```

```yaml
git add stacks/stack-1/
git commit -m "feat: add cert-manager, openfeature operator and platform flags"
git push
```

### 4. Uygulamalara Ajan (Sidecar) Enjekte Etme ve Test

OpenFeature Operatörü çalıştığında, uygulamalarınızın (`Deployment`) `template.metadata.annotations` kısmına şu iki satırı eklemeniz, o podun içine bayrakları okuyan bir `flagd` ajanının otomatik olarak yerleşmesi (*sidecar injection*) için yeterlidir:

```yaml
      annotations:
        openfeature.dev/enabled: "true"
        openfeature.dev/featureflagsource: "platform-flags"
```
