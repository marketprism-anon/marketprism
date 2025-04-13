#include <vector>
#include <xtensor/xtensor.hpp>
#include <xtensor/xadapt.hpp>
#include <xtensor/xarray.hpp>
#include <xtensor/xrandom.hpp>
#include <xtensor/xsort.hpp>
#include <cstdio>
#include <cstdlib>
#include <unistd.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <mutex>
#include <format>
#include <iostream>
#include <thread>

std::mutex mtx;

enum class Honesty{
    LAZY,
    ADVERSARY,
    HONEST,
};

class Utils {
public:
    static uint32_t get_random() {
       return arc4random();
    }
};


class Miner{
public:
    Miner(){}
    Miner(Miner& m): id{m.id}, honesty {m.honesty} {}
    Miner(const Miner& m): id{m.id}, honesty {m.honesty} {}
    Miner(Miner&& m): id{m.id}, honesty {m.honesty} {}
    Miner(const Miner&& m): id{m.id}, honesty {m.honesty} {}
    Miner(const uint32_t _id, double lazy_rate, double adversarial_rate) : id(_id) {
        const auto rnd = Utils::get_random()/pow(2.0, 32);
        if (rnd < adversarial_rate) {
            honesty = Honesty::ADVERSARY;
        } else if (rnd < adversarial_rate + lazy_rate) {
            honesty = Honesty::LAZY;
        } else {
            honesty = Honesty::HONEST;
        }
    };

    Miner& operator=(const Miner& rhs) {
        this->id = rhs.id;
        this->honesty = rhs.honesty;
        return *this;
    }

    bool operator<(const Miner& rhs) const {
        return id < rhs.id;
    }

    bool operator>(const Miner& rhs) const {
        return id > rhs.id;
    }

    bool operator==(const Miner& rhs) const {
        return id == rhs.id;
    }

    Honesty honesty;
    uint32_t id;

};
std::ostream& operator<<(std::ostream& os,  Miner& m) {
    std::string honesty;
    if (m.honesty == Honesty::HONEST ) {
        honesty = "honest";
    } else if (m.honesty == Honesty::LAZY ) {
        honesty = "lazy";
    } else {
        honesty = "adv";
    }
    return os << m.id << "/" << honesty;
};

std::tuple<double, double, double> CountStuff(xt::xarray<Miner> committee, uint32_t committeeSize) {
    double rateH = 0, rateA = 0, rateL = 0;
    for (uint32_t i = 0; i < committeeSize; i++) {
        if (committee.at(i).honesty == Honesty::HONEST){
            rateH += 1.0;
        } else if (committee.at(i).honesty == Honesty::LAZY) {
            rateL += 1.0;
        } else {
            rateA += 1.0;
        }
    }
    return std::make_tuple(rateH/committeeSize, rateA/committeeSize, rateL/committeeSize);
}

std::vector<Miner> xt_to_vector(xt::xarray<Miner> committee) {
    std::vector<Miner> output;
    for (uint32_t i = 0; i < committee.size(); i++) {
        output.push_back(committee.at(i));
    }
    return output;
}

std::vector<Miner> merge(std::vector<Miner> a, std::vector<Miner> b) {
    std::vector<Miner> output;
    for (uint32_t i = 0; i < a.size(); i++) {
        output.push_back(a.at(i));
    }
    for (uint32_t i = 0; i < b.size(); i++) {
        output.push_back(b.at(i));
    }
    return output;
}



void run_simulation(const std::vector<Miner>& popA,const std::vector<Miner>& popB, uint32_t committeeSize, uint32_t nTrials, double rateBucketA, double rateBucketB, FILE* f) {
    for (auto i = 0; i < nTrials; i++) {
        bool written = false;
        xt::xarray<Miner> minersA = xt::adapt(popA, {popA.size()});
        xt::xarray<Miner> minersB = xt::adapt(popB, {popB.size()});
        xt::random::shuffle(minersA);
        xt::random::shuffle(minersB);
        int j =0;
        auto countA = int(rateBucketA * committeeSize);
        auto countB = int(rateBucketB * committeeSize);
        while (minersA.size() >= countA && minersB.size() >= countB) {
            auto choiceA = xt::random::choice(minersA, countA, false);
            auto choiceB = xt::random::choice(minersB, countB, false);
            auto committeeA = xt_to_vector(choiceA);
            auto committeeB = xt_to_vector(choiceB);
            minersA = xt::setdiff1d (minersA, choiceA);
            minersB = xt::setdiff1d (minersB, choiceB);
            std::vector<Miner> dst = merge(committeeA, committeeB);

            auto committee = xt::adapt(dst, {dst.size()});
            auto shape2 = committee.shape();
            std::tuple<double, double, double> rates = CountStuff(committee, committeeSize);
            std::cout << "Rates: Honest = " << std::get<0>(rates) << " Adversary = " << std::get<1>(rates) << " Lazy = " << std::get<2>(rates)<<std::endl;
            auto honest = std::get<0>(rates);
            auto adversary = std::get<1>(rates);
            auto lazy = std::get<2>(rates);
            if (honest > 2.0/3) {
                mtx.lock();
                fprintf(f, "%d\n", j+1);
                fflush(f);
                written = true;
                mtx.unlock();
                break;
            } else if ( lazy + adversary >= 1.0/3 && lazy + adversary < 2.0/3 ) {
                int leader_changes = 0;
                elect_leader:
                uint32_t leader = Utils::get_random() % committeeSize;
                if (committee[leader].honesty != Honesty::ADVERSARY && lazy + honest > 2.0/3) {
                    mtx.lock();
                    fprintf(f, "%d\n", j+1);
                    fflush(f);
                    written = true;
                    mtx.unlock();
                    break;
                }
            }
            j++;
        }
        if (!written) {
            mtx.lock();
            fprintf(f, "fail\n");
            fflush(f);
            mtx.unlock();
        }
    }
}


int main(int argc, char *argv[]) {
    xt::random::seed(Utils::get_random());
    std::vector<std::thread> threads;
    uint32_t max_pop = 1e6;
    double lazy_rate = std::stod(argv[1]);
    double adversarial_rate = std::stod(argv[2]);
    uint32_t committeeSize = std::stoi(argv[3]);
    double adv_rate_in_A = std::stod(argv[4]);
    double rateBucketA = std::stod(argv[5]);
    double rateBucketB = 1 - rateBucketA;
    uint32_t nThreads = 10;
    uint32_t nIterations = 1000;
    std::vector<Miner> populationA;
    std::vector<Miner> populationB;
    for (uint32_t i = 0; i < max_pop/2; i++) {
        auto rateA = adv_rate_in_A * 2 * adversarial_rate;
        Miner m = Miner(i, lazy_rate, rateA);
        populationA.push_back(std::move(m));
    }
    for (uint32_t i = 0; i < max_pop/2; i++) {
        auto rateA = (1-adv_rate_in_A) * 2 * adversarial_rate;
        Miner m = Miner(i, lazy_rate, rateA);
        populationB.push_back(std::move(m));
    }
    char cmdFolder[256];
    std::sprintf(cmdFolder, "mkdir -p %.2f", rateBucketA);
    char fileName[256];
    system(cmdFolder);
    std::sprintf(fileName, "%.2f/weighted_%.2f:%d_lazy_%.2f_adv_%.2f.txt", rateBucketA, adv_rate_in_A, committeeSize, lazy_rate, adversarial_rate);
    FILE* f = fopen(fileName, "w");
    for (auto i = 0; i < nThreads; i++) {
        threads.push_back(std::thread([populationA, populationB, committeeSize, nIterations, f, rateBucketA, rateBucketB]() {
            run_simulation(populationA, populationB, committeeSize, nIterations, rateBucketA, rateBucketB, f);
        }));
    }
    for (auto i = 0; i < nThreads; i++) {
        threads.at(i).join();
    }
}