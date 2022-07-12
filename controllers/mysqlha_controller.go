/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paasappv1 "mysqlha/api/v1"
)

// MysqlhaReconciler reconciles a Mysqlha object
type MysqlhaReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=paasapp.emergen.cn,resources=mysqlhas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=paasapp.emergen.cn,resources=mysqlhas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=paasapp.emergen.cn,resources=mysqlhas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Mysqlha object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *MysqlhaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := r.Log.WithValues("mysqlha", req.NamespacedName)
	logger.Info("start reconcile")
	// TODO(user): your logic here
	mysqlha := &paasappv1.Mysqlha{}
	if err := r.Client.Get(ctx, req.NamespacedName, mysqlha); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	//configmap逻辑
	configMap := &v13.ConfigMap{}
	cm := newconfigmap(mysqlha)
	if err := controllerutil.SetControllerReference(mysqlha, cm, r.Scheme); err != nil {
		logger.Info("scale up failed: setcontrollerreference")
		return ctrl.Result{}, err
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: mysqlha.Namespace, Name: mysqlha.Name}, configMap); errors.IsNotFound(err) {
		logger.Info("found none configmap,create now")
		if err := r.Create(ctx, cm); err != nil {
			logger.Error(err, "create configmap failed")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}
	//service逻辑
	service := &v13.Service{}
	svc := newsvc(mysqlha)
	if err := controllerutil.SetControllerReference(mysqlha, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: mysqlha.Namespace, Name: mysqlha.Name}, service); errors.IsNotFound(err) {
		logger.Info("found none service,create now")
		if err := r.Create(ctx, svc); err != nil {
			logger.Error(err, "create service failed")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	//statefulset逻辑
	statefulSet := &v1.StatefulSet{}
	stateful := newstateful(mysqlha)
	if err := controllerutil.SetControllerReference(mysqlha, stateful, r.Scheme); err != nil {
		logger.Info("scale up failed: setcontrollerreference")
		return ctrl.Result{}, err
	}
	if err := r.Get(ctx, req.NamespacedName, statefulSet); errors.IsNotFound(err) {
		logger.Info("found none statefulset,create now")
		if err := r.Create(ctx, stateful); err != nil {
			logger.Error(err, "create statefulset failed")
			return ctrl.Result{}, err
		}
	} else if err == nil {
		if err := r.Update(ctx, stateful); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
func newconfigmap(cr *paasappv1.Mysqlha) *v13.ConfigMap {
	return &v13.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    map[string]string{"app": cr.Name},
		},
		Data: map[string]string{
			"master.cnf": "# Apply this config only on the master.\n[mysqld]\nlog-bin = mysql-bin\nbinlog_format = mixed\nrelay-log = relay-bin\nrelay-log-index = slave-relay-bin.index\nauto-increment-increment = 2\nauto-increment-offset = 1\ntransaction_isolation = READ-COMMITTED\nlog_timestamps=system\nrelay_log_recovery=ON\nmax_connections=2000\nmax_connect_errors=5000\nwait_timeout=864000\ninteractive_timeout=864000\nmax_allowed_packet=100M\ndefault_authentication_plugin=mysql_native_password\nthread_cache_size=64\ninnodb_buffer_pool_size=1G\nread_rnd_buffer_size=16M\nbulk_insert_buffer_size=100M\nsort_buffer_size=16M\njoin_buffer_size=16M\ninnodb_read_io_threads=4\ninnodb_write_io_threads=4\ndefault-time_zone = '+8:00'\nlower_case_table_names  = 1\n# Disabling symbolic-links is recommended to prevent assorted security risks\nsymbolic-links=0\n# Settings user and group are ignored when systemd is used.\n# If you need to run mysqld under a different user or group,\n# customize your systemd unit file for mariadb according to the\n# instructions in http://fedoraproject.org/wiki/Systemd\n[mysql]\ndefault-character-set = utf8mb4\n[client]\ndefault-character-set = utf8mb4",
			"slave.cnf":  "# Apply this config only on slaves.\n[mysqld]\nlog-bin = mysql-bin\nbinlog_format = mixed\nrelay-log = relay-bin\nrelay-log-index = slave-relay-bin.index\nauto-increment-increment = 2\nauto-increment-offset = 2\ntransaction_isolation = READ-COMMITTED\nlog_timestamps=system\nrelay_log_recovery=ON\nmax_connections=2000\nmax_connect_errors=5000\nwait_timeout=864000\ninteractive_timeout=864000\nmax_allowed_packet=100M\ndefault_authentication_plugin=mysql_native_password\nthread_cache_size=64\ninnodb_buffer_pool_size=1G\nread_rnd_buffer_size=16M\nbulk_insert_buffer_size=100M\nsort_buffer_size=16M\njoin_buffer_size=16M\ninnodb_read_io_threads=4\ninnodb_write_io_threads=4\ndefault-time_zone = '+8:00'\nlower_case_table_names  = 1\n# Disabling symbolic-links is recommended to prevent assorted security risks\nsymbolic-links=0\n# Settings user and group are ignored when systemd is used.\n# If you need to run mysqld under a different user or group,\n# customize your systemd unit file for mariadb according to the\n# instructions in http://fedoraproject.org/wiki/Systemd\n[mysql]\ndefault-character-set = utf8mb4\n[client]\ndefault-character-set = utf8mb4\n",
		},
	}
}

func newsvc(cr *paasappv1.Mysqlha) *v13.Service {
	return &v13.Service{
		ObjectMeta: v12.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    map[string]string{"app": cr.Name},
		},
		Spec: v13.ServiceSpec{
			Ports: []v13.ServicePort{
				{
					Name: "mysql",
					Port: 3306,
				},
			},
			ClusterIP: "None",
			Selector:  map[string]string{"app": cr.Name},
		},
	}
}

func newstateful(mysqlha *paasappv1.Mysqlha) *v1.StatefulSet {
	lbs := map[string]string{"app": mysqlha.Name}
	rootpd := mysqlha.Spec.RootPasswd
	scn := mysqlha.Spec.Storageclass
	repls := int32(mysqlha.Spec.Repl)
	return &v1.StatefulSet{
		ObjectMeta: v12.ObjectMeta{
			Name:      mysqlha.Name,
			Namespace: mysqlha.Namespace,
			Labels:    lbs,
		},
		Spec: v1.StatefulSetSpec{
			ServiceName: mysqlha.Name,
			Replicas:    &repls,
			Selector:    &v12.LabelSelector{MatchLabels: map[string]string{"app": mysqlha.Name}},
			Template: v13.PodTemplateSpec{
				ObjectMeta: v12.ObjectMeta{
					Labels: lbs,
				},
				Spec: v13.PodSpec{
					Affinity: &v13.Affinity{
						PodAntiAffinity: &v13.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v13.PodAffinityTerm{
								{
									LabelSelector: &v12.LabelSelector{
										MatchExpressions: []v12.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: v12.LabelSelectorOpIn,
												Values: []string{
													mysqlha.Name,
												},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Tolerations: []v13.Toleration{
						{
							Key:      "app",
							Operator: v13.TolerationOpEqual,
							Value:    "mysql",
							Effect:   v13.TaintEffectNoSchedule,
						},
					},
					InitContainers: []v13.Container{
						{
							Name:  "init-mysql",
							Image: "mysql:8.0.18",
							Command: []string{
								"bash",
								"-c",
								"set ex\n# 从hostname中获取索引，比如(mysql-1)会获取(1)\n[[ `hostname` =~ -([0-9]+)$ ]] || exit 1\nordinal=${BASH_REMATCH[1]}\necho [mysqld] > /mnt/conf.d/server-id.cnf\n          # 为了不让server-id=0而增加偏移量\necho server-id=$((100 + $ordinal)) >> /mnt/conf.d/server-id.cnf\n# 拷贝对应的文件到/mnt/conf.d/文件夹中\nif [[ $ordinal -eq 0 ]]; then\n  cp /mnt/config-map/master.cnf /mnt/conf.d/\nelse\n  cp /mnt/config-map/slave.cnf /mnt/conf.d/\nfi",
							},
							VolumeMounts: []v13.VolumeMount{
								{
									Name:      "conf",
									MountPath: "/mnt/conf.d",
								},
								{
									Name:      "config-map",
									MountPath: "/mnt/config-map",
								},
							},
						},
					},
					Containers: []v13.Container{
						{
							Name:  "mysql",
							Image: "mysql:8.0.18",
							Args: []string{
								"--default-authentication-plugin=mysql_native_password",
							},
							Env: []v13.EnvVar{
								{
									Name:  "MYSQL_ROOT_PASSWORD",
									Value: rootpd,
								},
							},
							Ports: []v13.ContainerPort{
								{
									Name:          "mysql",
									ContainerPort: 3306,
								},
							},
							VolumeMounts: []v13.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/mysql",
									SubPath:   "mysql",
								},
								{
									Name:      "conf",
									MountPath: "/etc/mysql/conf.d",
								},
							},
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    resource.MustParse("50m"),
									v13.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							LivenessProbe: &v13.Probe{
								Handler: v13.Handler{
									Exec: &v13.ExecAction{
										Command: []string{
											"mysqladmin",
											"ping",
										},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &v13.Probe{
								Handler: v13.Handler{
									Exec: &v13.ExecAction{
										Command: []string{
											"mysql",
											"-h",
											"127.0.0.1",
											"-uroot",
											"-p123456",
											"-e",
											"SELECT 1",
										},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       2,
								TimeoutSeconds:      1,
							},
						},
						{
							Name:  "xtrabackup",
							Image: "jstang/xtrabackup:2.3",
							Ports: []v13.ContainerPort{
								{
									Name:          "xtrabackup",
									ContainerPort: 3307,
								},
							},
							Env: []v13.EnvVar{
								{
									Name:  "ROOTPASSWORD",
									Value: rootpd,
								},
							},
							Command: []string{
								"bash",
								"-c",
								"set -ex\n          hostname=`hostname`\n\t\t  sleep 50\n          tmpuser=`mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e \"select user from mysql.user where user='$hostname' limit  1;\"`\n          if [[ ! $tmpuser ]];then\n            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e \"CREATE USER '$hostname'@'%' IDENTIFIED BY 'repl' ;\"\n            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e \"GRANT REPLICATION SLAVE ON *.* TO '$hostname'@'%' ;\"\n            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e \"FLUSH PRIVILEGES ;\"\n          fi\n          # 确定binlog 克隆数据位置(如果binlog存在的话).\n          cd /var/lib/mysql\n          # 如果存在该文件，则该xrabackup是从现有的从节点克隆出来的。\n          \n\t\tif [[ $hostname = mysqlha-1 ]];then\n\t\t\tbinlog0=`mysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD -e \"show master status\"| grep mysql|awk '{print $1}'`\n\t\t\tpostion0=`mysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD -e \"show master status\"| grep mysql|awk '{print $2}'`\n\t\t\tmysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD <<EOF\n\t\t\tstop slave;\n\t\t\treset slave;\n\t\t\tCHANGE MASTER TO master_log_file='$binlog0',\n\t\t\tmaster_log_pos=$postion0,\n\t\t\tMASTER_HOST='mysqlha-0.mysqlha',\n\t\t\tMASTER_USER='mysqlha-0',\n\t\t\tMASTER_PASSWORD='repl',\n\t\t\tMASTER_CONNECT_RETRY=10;\n\t\t\tSTART SLAVE;\nEOF\n\t\t\tbinlog1=`mysql -h mysqlha-1.mysqlha -uroot -p$ROOTPASSWORD -e \"show master status\"| grep mysql|awk '{print $1}'`\n\t\t\tpostion1=`mysql -h mysqlha-1.mysqlha -uroot -p$ROOTPASSWORD -e \"show master status\"| grep mysql|awk '{print $2}'`\n\t\t\tmysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD <<EOF\n\t\t\tstop slave;\n\t\t\treset slave;\n\t\t\tCHANGE MASTER TO master_log_file='$binlog1',\n\t\t\tmaster_log_pos=$postion1,\n\t\t\tMASTER_HOST='mysqlha-1.mysqlha',\n\t\t\tMASTER_USER='mysqlha-1',\n\t\t\tMASTER_PASSWORD='repl',\n\t\t\tMASTER_CONNECT_RETRY=10;\n\t\t\tSTART SLAVE;\nEOF\n\t\t\techo \"set master-slave successfully\"\n\t\tfi\n        \n\t\t  \n          exec ncat --listen --keep-open --send-only --max-conns=1 3307 -c \\\n            \"xtrabackup --backup --slave-info --stream=xbstream --host=127.0.0.1 --user=root\"",
							},
							VolumeMounts: []v13.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/mysql",
									SubPath:   "mysql",
								},
								{
									Name:      "conf",
									MountPath: "/etc/mysql/conf.d",
								},
							},
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    resource.MustParse("10m"),
									v13.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
						},
					},
					Volumes: []v13.Volume{
						{
							Name: "conf",
							VolumeSource: v13.VolumeSource{
								EmptyDir: &v13.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "config-map",
							VolumeSource: v13.VolumeSource{
								ConfigMap: &v13.ConfigMapVolumeSource{
									LocalObjectReference: v13.LocalObjectReference{
										Name: mysqlha.Name,
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []v13.PersistentVolumeClaim{
				{
					ObjectMeta: v12.ObjectMeta{
						Name:      "data",
						Namespace: mysqlha.Namespace,
					},
					Spec: v13.PersistentVolumeClaimSpec{
						AccessModes: []v13.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						StorageClassName: &scn,
						Resources: v13.ResourceRequirements{
							Requests: v13.ResourceList{
								v13.ResourceStorage: resource.MustParse("200Gi"),
							},
						},
					},
				},
			},
		},
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *MysqlhaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paasappv1.Mysqlha{}).
		Owns(&v1.StatefulSet{}).
		Owns(&v13.Service{}).
		Owns(&v13.ConfigMap{}).
		Complete(r)
}
