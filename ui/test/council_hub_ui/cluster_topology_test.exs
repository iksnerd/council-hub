defmodule CouncilHubUi.ClusterTopologyTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.ClusterTopology

  describe "build/2" do
    test "seeds still drive Epmd, with their node names as atoms" do
      assert [{:council_hub, epmd} | _] = ClusterTopology.build("a@10.0.0.1, b@10.0.0.2", nil)
      assert epmd[:strategy] == Cluster.Strategy.Epmd
      assert epmd[:config][:hosts] == [:"a@10.0.0.1", :"b@10.0.0.2"]
    end

    test "gossip runs alongside Epmd, so an address change is not fatal to the link" do
      topology = ClusterTopology.build("a@10.0.0.1", nil)

      assert Keyword.has_key?(topology, :council_hub)
      assert Keyword.has_key?(topology, :council_hub_gossip)
      assert topology[:council_hub_gossip][:strategy] == Cluster.Strategy.Gossip
    end

    test "gossip alone when no seeds are configured" do
      assert [{:council_hub_gossip, gossip}] = ClusterTopology.build(nil, nil)
      assert gossip[:strategy] == Cluster.Strategy.Gossip
      assert [{:council_hub_gossip, _}] = ClusterTopology.build("", nil)
    end

    test "COUNCIL_GOSSIP=0 turns multicast off without touching the seeds" do
      assert [{:council_hub, epmd}] = ClusterTopology.build("a@10.0.0.1", "0")
      assert epmd[:strategy] == Cluster.Strategy.Epmd
      assert ClusterTopology.build("", "false") == []
    end

    test "COUNCIL_GOSSIP=1 is the default, stated explicitly" do
      assert ClusterTopology.build("a@10.0.0.1", "1") == ClusterTopology.build("a@10.0.0.1", nil)
    end
  end
end
