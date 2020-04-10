> 假设已经存在一个`Ready`的`Kubernetes`环境。

# 安装部署

先下载源码至$GOPATH/src目录下，然后build镜像，之后开始部署admission webhook server

```bash
kubectl create ns test-admisssion
kubectl create -f deploy/admission-server-deploy.yaml
```

# 测试

## ValidatingWebhook测试
```bash
kubectl create -f deploy/validate-config.yaml
kubectl create -f deploy/test/test-admission-label-no.yaml
kubectl create -f deploy/test/test-admission-label-yes.yaml
```

## MutatingWebhook测试
```bash
kubectl create -f deploy/mutate-config.yaml
kubectl create -f deploy/test/test-admission-label-no.yaml
kubectl create -f deploy/test/test-admission-label-yes.yaml
```

有关一些更详细的说明可参考对应博文：https://www.jianshu.com/p/5248d85597df