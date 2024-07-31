/*
Copyright (c) 2020 Jian Zhang
Licensed under MIT https://github.com/jianz/jianz.github.io/blob/master/LICENSE
*/

package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
)

var (
	etcdCA, etcdCert, etcdKey, etcdHost string
	etcdPort                            int

	k8sKeyPrefix string
	nsName       string

	cmd = &cobra.Command{
		Use:   "resetns [flags] <namespace name>",
		Short: "Reset the Terminating Namespace back to Active status.",
		Long:  "Reset the Terminating Namespace back to Active status.\n",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one namespace name argument")
			}
			nsName = args[0]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := resetNS()
			return err
		},
	}
)

// Execute reset the Terminating Namespace to Bound status.
func Execute() {
	cmd.Flags().StringVar(&etcdCA, "etcd-ca", "ca.crt", "CA Certificate used by etcd")
	cmd.Flags().StringVar(&etcdCert, "etcd-cert", "etcd.crt", "Public key used by etcd")
	cmd.Flags().StringVar(&etcdKey, "etcd-key", "etcd.key", "Private key used by etcd")
	cmd.Flags().StringVar(&etcdHost, "etcd-host", "localhost", "The etcd domain name or IP")
	cmd.Flags().StringVar(&k8sKeyPrefix, "k8s-key-prefix", "registry", "The etcd key prefix for kubernetes resources.")
	cmd.Flags().IntVar(&etcdPort, "etcd-port", 2379, "The etcd port number")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func resetNS() error {
	etcdCli, err := etcdClient()
	if err != nil {
		return fmt.Errorf("cannot connect to etcd: %v", err)
	}
	defer etcdCli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return recoverNS(ctx, etcdCli)
}

func etcdClient() (*clientv3.Client, error) {
	ca, err := ioutil.ReadFile(etcdCA)
	if err != nil {
		return nil, err
	}

	keyPair, err := tls.LoadX509KeyPair(etcdCert, etcdKey)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)

	return clientv3.New(clientv3.Config{
		Endpoints:   []string{fmt.Sprintf("%s:%d", etcdHost, etcdPort)},
		DialTimeout: 2 * time.Second,
		TLS: &tls.Config{
			RootCAs:            certPool,
			Certificates:       []tls.Certificate{keyPair},
			InsecureSkipVerify: true,
		},
	})
}

func recoverNS(ctx context.Context, client *clientv3.Client) error {

	gvk := schema.GroupVersionKind{Group: v1.GroupName, Version: "v1", Kind: "Namespace"}
	ns := &v1.Namespace{}

	runtimeScheme := runtime.NewScheme()
	runtimeScheme.AddKnownTypeWithName(gvk, ns)
	protoSerializer := protobuf.NewSerializer(runtimeScheme, runtimeScheme)

	// Get NS value from etcd which in protobuf format
	key := fmt.Sprintf("/%s/namespaces/%s", k8sKeyPrefix, nsName)
	resp, err := client.Get(ctx, key)
	if err != nil {
		return err
	}

	if len(resp.Kvs) < 1 {
		return fmt.Errorf("cannot find namespace [%s] in etcd with key [%s]\nplease check the k8s-key-prefix and the namespace name are set correctly", nsName, key)
	}

	// Decode protobuf value to NS struct
	_, _, err = protoSerializer.Decode(resp.Kvs[0].Value, &gvk, ns)
	if err != nil {
		return err
	}

	// Set Namespace status from Terminating to Active by removing value of DeletionTimestamp, DeletionGracePeriodSeconds, and modifying the status phase and conditions
	if (*ns).ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("namespace [%s] is not in terminating status", nsName)
	}
	(*ns).ObjectMeta.DeletionTimestamp = nil
	(*ns).ObjectMeta.DeletionGracePeriodSeconds = nil
	(*ns).Status.Phase = v1.NamespaceActive
	(*ns).Status.Conditions = []v1.NamespaceCondition{}

	// Encode fixed NS struct to protobuf value
	var fixedNS bytes.Buffer
	err = protoSerializer.Encode(ns, &fixedNS)
	if err != nil {
		return err
	}

	// Write the updated protobuf value back to etcd
	client.Put(ctx, key, fixedNS.String())
	return nil
}
