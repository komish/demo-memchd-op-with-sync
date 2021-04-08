# Memcached Controller w/ Additional Replica Sync Controller

This is a quick proof of concept that combines the OperatorSDK's Memcached
Operator example with an additional controller that synchronizes the size of the
memcached installation based on a Deployment's replica within the same
namespace.

The new code is at
[controllers/deployments_sync_controller.go](controllers/deployments_sync_controller.go).

## Additional Controller Implementation

The additional controller watches `appv1.Deployment` resources. When a
Deployment instance changes, the controller checks that deployment for the
existence of a label `memcached-operator/associated-memcached-deployment-name`.

The expected value of that label is the name of an instance of the `Memcached`
resource in the same namespace as the deployment. If the key does not exist, or
the value references a deployment that does not exist, the controller does
nothing.

If the instance of `Memcached` exists, the controller will compare the
`Memcached.Spec.Size` value to the `Deployments.Spec.Replicas` of the deployment
that triggered the event. If they match, the controller does nothing.

If they do not match, the controller will change the `Memcached.Spec.Size` value
to match the `Deployments.Spec.Replicas`.

## Example execution

To test, clone this repo, change directory into it, and then run these commands:

```shell
make install # install the memcached
make run # run the controller

# in another window
oc apply -f config/samples/cache_v1alpha1_memcached.yaml # create a memcached with 1 replica
oc apply -f config/samples/deployment.yaml # create a deployment to use as a sync target for the memcached


oc get deployments -w # watch for changes to the memcached deployment as well as the sync target
```

Sample controller log

```
...
2021-04-08T15:01:46.676-0500    INFO    controllers.Memcached   Creating a new Deployment       {"memcached": "memcached-testing/memcached-sample", "Deployment.Namespace": "memcached-testing", "Deployment.Name": "memcached-sample"}
2021-04-08T15:02:16.308-0500    INFO    controllers.DeploymentSync      Replica count on deployment has changed. Syncing memcached      {"deploymentSync": "memcached-testing/sync-target-deployment", "memcached-identifier": "memcached-testing/memcached-sample", "from": 1, "to": 4}
...
```

Sample Watch output (last command)

```
NAME                     READY   UP-TO-DATE   AVAILABLE   AGE
memcached-sample         1/1     1            1           28s
sync-target-deployment   0/4     0            0           0s
sync-target-deployment   0/4     0            0           0s
sync-target-deployment   0/4     0            0           0s
sync-target-deployment   0/4     4            0           0s
memcached-sample         1/4     1            1           30s
memcached-sample         1/4     1            1           30s
memcached-sample         1/4     1            1           30s
memcached-sample         1/4     4            1           30s
sync-target-deployment   1/4     4            1           3s
sync-target-deployment   2/4     4            2           3s
memcached-sample         2/4     4            2           33s
memcached-sample         3/4     4            3           33s
sync-target-deployment   3/4     4            3           3s
memcached-sample         4/4     4            4           33s
sync-target-deployment   4/4     4            4           3s
```

Additional scale test with watch:

```shell
# oc scale deployment sync-target-deployment --replicas=2 ; oc get deployments -w
deployment.apps/sync-target-deployment scaled
NAME                     READY   UP-TO-DATE   AVAILABLE   AGE
memcached-sample         2/2     2            2           4m30s
sync-target-deployment   2/2     2            2           4m
```

## Challenges

- In this particular implementation, you would have an issue if a user then went
  and scaled the `Memcached` instance manually. You would solve need to either
  perform a periodic sync, or find some way to more-closely couple the Memcached
  instance to the deployment.

- In this particular implementation, you would have an issue if multiple
  deployments indicated the same `Memcached` instance needed to be synced
  according to its replicas. The controller has no way of determining which one
  should win, and therefore the controller would constantly be adjusting the
  `Memcached` resource.

Ideally, you would want to have a closer binding between the Memcached instance
and the external deployment - either via some kind of reference in the
`Memcached` spec, some additional API, or some configuration that allows you to
look up the relationship between the two.