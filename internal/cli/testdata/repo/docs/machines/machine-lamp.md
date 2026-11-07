# Lamp

This page covers every marker kind.

<!-- docgen:config pkg=machine type=Config -->
<!-- /docgen:config -->

The default wait is <!-- docgen:const pkg=machine name=DefaultWait --><!-- /docgen:const --> and at most <!-- docgen:const pkg=machine name=MaxTries --><!-- /docgen:const --> tries.

<!-- docgen:states pkg=machine type=Lamp -->
<!-- /docgen:states -->

<!-- docgen:states pkg=machine type=Counter -->
<!-- /docgen:states -->

<!-- docgen:errors pkg=machine type=Lamp -->
<!-- /docgen:errors -->

<!-- docgen:states pkg=machine type=SideState -->
<!-- /docgen:states -->

The period is <!-- docgen:const pkg=machine name=period --><!-- /docgen:const --> and the hold <!-- docgen:const pkg=machine name=hold --><!-- /docgen:const -->.

<!-- docgen:doc pkg=machine section=Rates -->
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine name=Lamp.Start -->
<!-- /docgen:doc -->

<!-- docgen:code file=v2/machine/machine.go lines=96-102 -->
<!-- /docgen:code -->

<!-- docgen:output test=machine.ExampleLamp -->
<!-- /docgen:output -->

<!-- docgen:output cmd=v2/examples/hello -->
<!-- /docgen:output -->

<!-- docgen:code file=v2/machine/example_test.go symbol=ExampleLamp -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Tick -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideState -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideB -->
<!-- /docgen:code -->

It needs Go <!-- docgen:gomod field=go --><!-- /docgen:gomod --> or later.

<!-- docgen:usage cmd=cmd/tool -->
<!-- /docgen:usage -->

<!-- docgen:doc pkg=cmd/tool section=Flags -->
<!-- /docgen:doc -->

<!-- docgen:code file=v2/examples/hello/main.go -->
<!-- /docgen:code -->
